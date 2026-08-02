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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	infrastructurev1alpha1 "github.com/thebhdn/cluster-api-provider-libvirt/api/v1alpha1"
	"github.com/thebhdn/cluster-api-provider-libvirt/internal/libvirtclient"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var _ = Describe("LibvirtMachine Controller", func() {
	const (
		resourceName    = "test-machine"
		testClusterName = "test-cluster"
		testNamespace   = "default"
	)

	var (
		ctx            context.Context
		namespacedName types.NamespacedName
		machine        *infrastructurev1alpha1.LibvirtMachine
		cluster        *infrastructurev1alpha1.LibvirtCluster
	)

	BeforeEach(func() {
		ctx = context.Background()
		namespacedName = types.NamespacedName{
			Name:      resourceName,
			Namespace: testNamespace,
		}

		// Create LibvirtCluster first
		cluster = &infrastructurev1alpha1.LibvirtCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testClusterName + "-infra",
				Namespace: testNamespace,
			},
			Spec: infrastructurev1alpha1.LibvirtClusterSpec{
				URI: "qemu+tcp://localhost/system",
			},
		}
		err := k8sClient.Create(ctx, cluster)
		Expect(err).NotTo(HaveOccurred())

		// Create LibvirtMachine directly — no CAPI Machine/Cluster required
		machine = &infrastructurev1alpha1.LibvirtMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: testNamespace,
			},
			Spec: infrastructurev1alpha1.LibvirtMachineSpec{
				Image: "/var/lib/libvirt/images/base.qcow2",
				VCPU:  2,
			},
		}
		err = k8sClient.Create(ctx, machine)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		toDelete := &infrastructurev1alpha1.LibvirtMachine{}
		err := k8sClient.Get(ctx, namespacedName, toDelete)
		if err != nil && !apierrors.IsNotFound(err) {
			Expect(err).NotTo(HaveOccurred())
		}
		if apierrors.IsNotFound(err) {
			return
		}
		err = k8sClient.Delete(ctx, toDelete)
		Expect(err).NotTo(HaveOccurred())

		// Also delete the cluster
		clusterDel := &infrastructurev1alpha1.LibvirtCluster{}
		err = k8sClient.Get(ctx, types.NamespacedName{Name: testClusterName + "-infra", Namespace: testNamespace}, clusterDel)
		if err == nil {
			_ = k8sClient.Delete(ctx, clusterDel)
		}
	})

	DescribeTable(
		"reconciliation with mock provider",
		func(machineState libvirtclient.DomainState, mockErr error, expectReady bool, expectProvisioned bool) {
			By("calling reconcileNormal")
			// Get the updated LibvirtMachine
			updated := &infrastructurev1alpha1.LibvirtMachine{}
			Eventually(func() bool {
				return k8sClient.Get(ctx, namespacedName, updated) == nil
			}, "10s", "1s").Should(BeTrue())

			// Get the LibvirtCluster
			clusterUpdated := &infrastructurev1alpha1.LibvirtCluster{}
			Eventually(func() bool {
				return k8sClient.Get(ctx, types.NamespacedName{Name: testClusterName + "-infra", Namespace: testNamespace}, clusterUpdated) == nil
			}, "10s", "1s").Should(BeTrue())

			// Construct a MachineScope for direct reconcileNormal call
			infraReady := true
			// Pre-add finalizer so reconcileNormal skips the early return at line 195
			controllerutil.AddFinalizer(updated, infrastructurev1alpha1.LibvirtMachineFinalizer)
			scope := &MachineScope{
				Cluster: &clusterv1.Cluster{
					Status: clusterv1.ClusterStatus{
						Initialization: clusterv1.ClusterInitializationStatus{
							InfrastructureProvisioned: &infraReady,
						},
					},
				},
				Machine:        &clusterv1.Machine{},
				LibvirtCluster: clusterUpdated,
				LibvirtMachine: updated,
				Ctx:            ctx,
				MachineConfig:  newMachineConfig(updated, clusterUpdated),
			}

			// Create reconciler with mock provider
			reconciler := &LibvirtMachineReconciler{
				Client:          k8sClient,
				Scheme:          k8sClient.Scheme(),
				MachineProvider: &MockMachineProvider{},
			}
			mockProvider := reconciler.MachineProvider.(*MockMachineProvider)
			mockProvider.SetMachineState(machineState)
			mockProvider.SetGetMachineStateErr(mockErr)

			// Call reconcileNormal directly (bypasses owner-check in Reconcile entry point)
			result, err := reconciler.reconcileNormal(scope)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeZero())

			// Verify status was updated
			Expect(updated.Status.Ready).To(Equal(expectReady), "status.ready mismatch")
			Expect(updated.Status.Initialization.Provisioned).To(Equal(expectProvisioned), "status.initialization.provisioned mismatch")

			// Verify condition was set appropriately
			if expectReady && machineState == libvirtclient.DomainStateRunning {
				Expect(updated.Status.Conditions).NotTo(BeEmpty())
				found := false
				for _, c := range updated.Status.Conditions {
					if c.Type == infrastructurev1alpha1.DomainRunningCondition {
						Expect(c.Status).To(Equal(metav1.ConditionTrue))
						found = true
					}
				}
				Expect(found).To(BeTrue(), "DomainRunning condition should be True when machine is running")
			}
		},
		Entry("domain running -> ready and provisioned", libvirtclient.DomainStateRunning, nil, true, true),
		Entry("domain stopped -> not ready but provisioned", libvirtclient.DomainStateStopped, nil, false, true),
		// Entry("domain not found -> ready and provisioned after creation", libvirtclient.DomainStateNotFound, nil, true, true),
	)

	It("should handle deletion correctly", func() {
		By("calling reconcileDelete")
		updated := &infrastructurev1alpha1.LibvirtMachine{}
		Expect(k8sClient.Get(ctx, namespacedName, updated)).To(Succeed())
		// Add finalizer — reconcileDelete expects it to be present
		controllerutil.AddFinalizer(updated, infrastructurev1alpha1.LibvirtMachineFinalizer)
		Expect(k8sClient.Update(ctx, updated)).To(Succeed())

		// Get the LibvirtCluster
		clusterUpdated := &infrastructurev1alpha1.LibvirtCluster{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: testClusterName + "-infra", Namespace: testNamespace}, clusterUpdated)).To(Succeed())

		// Construct a MachineScope for direct reconcileDelete call
		scope := &MachineScope{
			Cluster:        &clusterv1.Cluster{},
			Machine:        &clusterv1.Machine{},
			LibvirtCluster: clusterUpdated,
			LibvirtMachine: updated,
			Ctx:            ctx,
			MachineConfig:  newMachineConfig(updated, clusterUpdated),
		}

		// Create reconciler with mock provider
		reconciler := &LibvirtMachineReconciler{
			Client:          k8sClient,
			Scheme:          k8sClient.Scheme(),
			MachineProvider: &MockMachineProvider{},
		}

		// Call reconcileDelete directly (bypasses owner-check in Reconcile entry point)
		result, err := reconciler.reconcileDelete(scope)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(BeZero())
	})
})
