package controllers

import (
	"context"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	databasev1 "github.com/pkpivot/pg-simple-operator/api/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

var _ = Describe("postgresql_controller", func() {
	const pgName = "pg1"
	fmt.Println("Describe")
	It("Should create matching pod and clean up", func() {
		By("Creating a Postgresql object")
		ctx := context.Background()
		pg := databasev1.Postgresql{
			TypeMeta:   metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{Name: pgName, Namespace: "default"},
			Spec: databasev1.PostgresqlSpec{
				DefaultUser: "pguser",
				Password:    "password1!",
			},
			Status: databasev1.PostgresqlStatus{},
		}
		Expect(k8sClient.Create(ctx, &pg)).Should(Succeed())
		var pod v1.Pod
		Eventually(func() bool {
			if err := k8sClient.Get(ctx, GetPodNamespacedName(pg), &pod); err != nil {
				return false
			}
			return true
		}).WithTimeout(120 * time.Second).WithPolling(time.Second).Should(BeTrue())

		By("postgres object state change to running")
		var retrievedPg databasev1.Postgresql
		Eventually(func() bool {
			if err := k8sClient.Get(ctx, GetPodNamespacedName(pg), &retrievedPg); err != nil {
				return false
			}
			if retrievedPg.Status.Phase == databasev1.PgUp {
				return true
			}
			return false
		}).WithTimeout(600 * time.Second).WithPolling(time.Second).Should(BeTrue())

		By("deleting custom resource")
		Expect(k8sClient.Delete(ctx, &pg)).Should(Succeed())

		Eventually(func() bool {
			err := k8sClient.Get(ctx, GetPodNamespacedName(pg), &pod)
			if err != nil {
				if client.IgnoreNotFound(err) == nil {
					return true
				}
			}
			return false
		}).WithTimeout(600 * time.Second).WithPolling(time.Second).Should(BeTrue())
	})
})
