/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	databasev1 "github.com/pkpivot/pg-simple-operator/api/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"
)

const postgresImage = "postgres:14.5"

const postgresqlFinalizer = "database.db.example.com/finalizer"

// PostgresqlReconciler reconciles a Postgresql object
type PostgresqlReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=database.db.example.com,resources=postgresqls,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=database.db.example.com,resources=postgresqls/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=database.db.example.com,resources=postgresqls/finalizers,verbs=update

// Permissions to access Pods

//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;create;update;delete;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Postgresql object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *PostgresqlReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Retrieve the Postgresql object named in the reques
	var pg databasev1.Postgresql
	err := r.Get(ctx, req.NamespacedName, &pg)
	if err != nil {
		logger.Error(err, "Could not retrieve postgresql object")
		// ignore not found errors - could be caused because the object is deleting
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var pod v1.Pod

	// If no corresponding pod exists, create one
	if err := r.Get(ctx, req.NamespacedName, &pod); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, err
		}

		// A notFound error means we should create a pod
		podSpec := createPodSpec(pg)

		pod.Spec = podSpec
		pod.Name = pg.Name
		pod.Namespace = pg.Namespace
		if err := r.Create(ctx, &pod); err != nil {
			logger.Error(err, "could not create pod")
			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		}
	}

	// Update the status of the postgresql object based on the status of the Pod
	switch pod.Status.Phase {
	case v1.PodPending:
		pg.Status.Phase = databasev1.PgPending
	case v1.PodRunning:
		pg.Status.Phase = databasev1.PgUp
	default:
		pg.Status.Phase = databasev1.PgFailed
	}
	r.Status().Update(ctx, &pg)

	if result, err := r.registerFinalizer(ctx, &pg); err != nil {
		logger.Error(err, "Could not ergister finalizer")
		return result, err
	}

	if objectDeleting(&pg) {
		err := r.deleteExternalResources(ctx, &pg)
		return ctrl.Result{}, err
	}

	logger.Info("Status ", "name", pod.Name, "pod phase ", pod.Status.Phase, "Pg phase", pg.Status.Phase)

	return ctrl.Result{RequeueAfter: time.Second * 5}, nil
}

func (r *PostgresqlReconciler) deleteExternalResources(ctx context.Context, pg *databasev1.Postgresql) error {
	var pod v1.Pod
	logger := log.FromContext(ctx)
	if controllerutil.ContainsFinalizer(pg, postgresqlFinalizer) {
		// our finalizer is present, so lets handle any external dependency
		if err := r.Get(ctx, GetPodNamespacedName(*pg), &pod); err == nil {
			var policy metav1.DeletionPropagation
			policy = metav1.DeletePropagationForeground
			if err := r.Delete(ctx, &pod, &client.DeleteOptions{PropagationPolicy: &policy}); err != nil {
				logger.Error(err, "Could not delete pod")
				return err
			}
		}
	}
	// remove our finalizer from the list and update it.
	controllerutil.RemoveFinalizer(pg, postgresqlFinalizer)
	return r.Update(ctx, pg)
}

func createPodSpec(db databasev1.Postgresql) v1.PodSpec {
	const dbDisk = "postgresql-db-disk"
	container := v1.Container{
		Name:  getPodName(db),
		Image: postgresImage,
		Ports: []v1.ContainerPort{{ContainerPort: 5432}},
		Env: []v1.EnvVar{{Name: "POSTGRES_PASSWORD", Value: db.Spec.Password},
			{Name: "PGDATA", Value: "/data/pgdata"}},
		VolumeMounts: []v1.VolumeMount{{Name: dbDisk, MountPath: "/data"}},
	}

	result := v1.PodSpec{
		Containers: []v1.Container{container},
		// TODO - replace with persistentvolume claim
		// default to emptydir for now
		Volumes: []v1.Volume{{Name: dbDisk}},
	}
	return result
}

func getPodName(pg databasev1.Postgresql) string {
	return pg.Name
}

func GetPodNamespacedName(pg databasev1.Postgresql) types.NamespacedName {
	return types.NamespacedName{
		Name:      getPodName(pg),
		Namespace: pg.Namespace,
	}
}

func (r *PostgresqlReconciler) registerFinalizer(ctx context.Context, pg *databasev1.Postgresql) (ctrl.Result, error) {
	var err error

	// examine DeletionTimestamp to determine if object is under deletion
	if !objectDeleting(pg) {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(pg, postgresqlFinalizer) {
			controllerutil.AddFinalizer(pg, postgresqlFinalizer)
			err = r.Update(ctx, pg)
		}
	}
	return ctrl.Result{}, err
}

func objectDeleting(pg *databasev1.Postgresql) bool {
	return !pg.ObjectMeta.DeletionTimestamp.IsZero()
}

// SetupWithManager sets up the controller with the Manager.
func (r *PostgresqlReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&databasev1.Postgresql{}).
		Complete(r)
}
