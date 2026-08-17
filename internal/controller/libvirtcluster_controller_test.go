/*
Copyright 2026 Bohdan Leshchenko.

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

package controller

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	infrastructurev1alpha1 "github.com/thebhdn/cluster-api-provider-libvirt/api/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

var _ = Describe("LibvirtCluster Controller", func() {
	const (
		resourceName  = "test-cluster"
		testNamespace = "default"
	)

	var (
		ctx            context.Context
		namespacedName types.NamespacedName
		cluster        *infrastructurev1alpha1.LibvirtCluster
	)

	BeforeEach(func() {
		ctx = context.Background()
		namespacedName = types.NamespacedName{
			Name:      resourceName,
			Namespace: testNamespace,
		}

		// Create the LibvirtCluster directly — no CAPI owner required
		cluster = &infrastructurev1alpha1.LibvirtCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:       resourceName,
				Namespace:  testNamespace,
				Finalizers: []string{infrastructurev1alpha1.LibvirtClusterFinalizer},
			},
			Spec: infrastructurev1alpha1.LibvirtClusterSpec{
				ControlPlaneEndpoint: clusterv1.APIEndpoint{
					Host: "localhost",
					Port: 6443,
				},
			},
		}
		err := k8sClient.Create(ctx, cluster)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		toDelete := &infrastructurev1alpha1.LibvirtCluster{}
		err := k8sClient.Get(ctx, namespacedName, toDelete)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return
			}
			Expect(err).NotTo(HaveOccurred())
		}

		// Force-remove finalizer so Delete works without a controller
		if err := k8sClient.Get(ctx, namespacedName, toDelete); err != nil {
			if apierrors.IsNotFound(err) {
				return
			}
			Expect(err).NotTo(HaveOccurred())
		}
		toDelete.Finalizers = nil
		Expect(k8sClient.Update(ctx, toDelete)).To(Succeed())

		Expect(k8sClient.Delete(ctx, toDelete)).To(Succeed())
		Eventually(func() error {
			obj := &infrastructurev1alpha1.LibvirtCluster{}
			return k8sClient.Get(ctx, namespacedName, obj)
		}, "5s", "200ms").ShouldNot(Succeed())
	})

	DescribeTable(
		"reconciliation with mock provider",
		func(mockErr error, expectReady bool, expectProvisioned bool) {
			By("calling reconcileNormal")
			// Reconcile the LibvirtCluster
			updated := &infrastructurev1alpha1.LibvirtCluster{}
			Eventually(func() bool {
				return k8sClient.Get(ctx, namespacedName, updated) == nil
			}, "10s", "1s").Should(BeTrue())

			// Construct a ClusterScope for direct reconcileNormal call
			scope := &ClusterScope{
				Cluster:        &clusterv1.Cluster{}, // minimal; reconcileNormal doesn't use it
				LibvirtCluster: updated,
				InfraConfig:    newInfraConfig(updated),
				Ctx:            ctx,
			}

			// Create reconciler with mock provider
			reconciler := &LibvirtClusterReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Provider: &MockProvider{},
			}
			reconciler.Provider.(*MockProvider).SetEnsureInfraErr(mockErr)

			// Call reconcileNormal directly (bypasses owner-check in Reconcile entry point)
			result, err := reconciler.reconcileNormal(scope)
			Expect(err).NotTo(HaveOccurred())
			if expectReady {
				Expect(result.RequeueAfter).To(BeZero(), "reconcileNormal should not request requeue on success")
			} else {
				Expect(result.RequeueAfter).To(Equal(requeueTimeShort), "reconcileNormal should requeue on EnsureInfra failure")
			}

			// Verify status was updated via the patch helper
			Expect(updated.Status.Ready).To(Equal(expectReady), "status.ready mismatch")
			Expect(updated.Status.Initialization.Provisioned).To(Equal(expectProvisioned), "status.initialization.provisioned mismatch")

			// Verify condition was set
			if expectReady {
				Expect(updated.Status.Conditions).NotTo(BeEmpty())
				found := false
				for _, c := range updated.Status.Conditions {
					if c.Type == infrastructurev1alpha1.InfrastructureReadyCondition {
						Expect(c.Status).To(Equal(metav1.ConditionTrue))
						found = true
					}
				}
				Expect(found).To(BeTrue(), "InfrastructureReady condition should be True")
			}
		},
		Entry("EnsureInfra success -> ready and provisioned", nil, true, true),
		Entry("EnsureInfra error -> not ready and not provisioned", errors.New("mock ensure infra failure"), false, false),
	)

	It("should handle deletion correctly", func() {
		By("calling reconcileDelete")
		updated := &infrastructurev1alpha1.LibvirtCluster{}
		Expect(k8sClient.Get(ctx, namespacedName, updated)).To(Succeed())

		// Construct a ClusterScope for direct reconcileDelete call
		scope := &ClusterScope{
			Cluster:        &clusterv1.Cluster{},
			LibvirtCluster: updated,
			Ctx:            ctx,
			InfraConfig:    newInfraConfig(updated),
		}

		// Create reconciler with mock provider
		reconciler := &LibvirtClusterReconciler{
			Client:   k8sClient,
			Scheme:   k8sClient.Scheme(),
			Provider: &MockProvider{},
		}

		// Call reconcileDelete directly (bypasses owner-check in Reconcile entry point)
		result, err := reconciler.reconcileDelete(scope)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(BeZero())
	})
})
