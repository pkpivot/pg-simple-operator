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
	"fmt"
	databasev1 "github.com/pkpivot/pg-simple-operator/api/v1"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"
)

const postgresImage = "postgres:14.5"

// PostgresqlReconciler reconciles a Postgresql object
type PostgresqlReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=database.db.example.vmware.com,resources=postgresqls,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=database.db.example.vmware.com,resources=postgresqls/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=database.db.example.vmware.com,resources=postgresqls/finalizers,verbs=update

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

	var pg databasev1.Postgresql
	err := r.Get(ctx, req.NamespacedName, &pg)
	if err != nil {
		logger.Error(err, "Could not retrieve postgresql object")
		return ctrl.Result{}, err
	}

	// Look for stateful set
	// var statefulSet apps.StatefulSet
	var pod v1.Pod

	if err := r.Get(ctx, req.NamespacedName, &pod); err != nil {
		fmt.Printf("%T", err)
		if client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, err
		}

		// Now create the pod
		podSpec := createPodSpec(pg)

		pod.Spec = podSpec
		pod.Name = pg.Name
		pod.Namespace = pg.Namespace
		if err := r.Create(ctx, &pod); err != nil {
			logger.Error(err, "could not create pod")
			return ctrl.Result{RequeueAfter: time.Second * 2}, nil
		}
	}

	switch pod.Status.Phase {
	case v1.PodPending:
		pg.Status.Phase = databasev1.PgPending
	case v1.PodRunning:
		pg.Status.Phase = databasev1.PgUp
	default:
		pg.Status.Phase = databasev1.PgFailed
	}

	logger.Info("Status ", "name", pod.Name, "pod phase ", pod.Status.Phase, "Pg phase", pg.Status.Phase)

	return ctrl.Result{RequeueAfter: time.Second * 2}, nil
}

func createPodSpec(db databasev1.Postgresql) v1.PodSpec {
	const dbDisk = "postgresql-db-disk"
	container := v1.Container{
		Name:  db.Name,
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

// TODO - Complete stateful set spec.
func constructStatefulSet(db databasev1.Postgresql) (*apps.StatefulSet, error) {
	name := db.Name

	var i int32 = 1
	spec := apps.StatefulSetSpec{
		Replicas: &i,
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"app": name},
			MatchExpressions: []metav1.LabelSelectorRequirement{{
				"app", metav1.LabelSelectorOpExists, []string{}},
			},
		},
	}

	statefulSet := &apps.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
			Name:        name,
			Namespace:   db.Namespace,
		},
		Spec: spec,
	}
	return statefulSet, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PostgresqlReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&databasev1.Postgresql{}).
		Complete(r)
}
