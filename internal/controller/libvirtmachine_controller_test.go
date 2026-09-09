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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
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
		secret         *corev1.Secret
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
				ControlPlaneEndpoint: clusterv1.APIEndpoint{
					Host: "localhost",
					Port: 6443,
				},
			},
		}
		err := k8sClient.Create(ctx, cluster)
		Expect(err).NotTo(HaveOccurred())

		// Create UserData Secret first
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testClusterName + "-secret",
				Namespace: testNamespace,
			},
			StringData: map[string]string{
				"value": "be metal my friend",
			},
		}
		err = k8sClient.Create(ctx, secret)
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
		machineDel := &infrastructurev1alpha1.LibvirtMachine{}
		err := k8sClient.Get(ctx, namespacedName, machineDel)
		if err != nil && !apierrors.IsNotFound(err) {
			Expect(err).NotTo(HaveOccurred())
		}
		if apierrors.IsNotFound(err) {
			return
		}

		// A finalizer blocks fake-client deletion (deletionTimestamp stays
		// set, object lingers), which poisons the next spec's BeforeEach.
		// Strip it so the object is actually removed.
		if len(machineDel.GetFinalizers()) > 0 {
			machineDel.SetFinalizers(nil)
			err = k8sClient.Update(ctx, machineDel)
			Expect(err).NotTo(HaveOccurred())
		}
		err = k8sClient.Delete(ctx, machineDel)
		Expect(err).NotTo(HaveOccurred())

		secretDel := &corev1.Secret{}
		err = k8sClient.Get(ctx, types.NamespacedName{Name: testClusterName + "-secret", Namespace: testNamespace}, secretDel)
		if err == nil {
			_ = k8sClient.Delete(ctx, secretDel)
		}

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

			// Get the libvMachine LibvirtMachine
			libvMachine := &infrastructurev1alpha1.LibvirtMachine{}
			Eventually(func() bool {
				return k8sClient.Get(ctx, namespacedName, libvMachine) == nil
			}, "10s", "1s").Should(BeTrue())

			userDataSecret := &corev1.Secret{}
			Eventually(func() bool {
				return k8sClient.Get(ctx, types.NamespacedName{Name: testClusterName + "-secret", Namespace: testNamespace}, userDataSecret) == nil
			}, "10s", "1s").Should(BeTrue())

			// Get the LibvirtCluster
			libvCluster := &infrastructurev1alpha1.LibvirtCluster{}
			Eventually(func() bool {
				return k8sClient.Get(ctx, types.NamespacedName{Name: testClusterName + "-infra", Namespace: testNamespace}, libvCluster) == nil
			}, "10s", "1s").Should(BeTrue())

			// Construct a MachineScope for direct reconcileNormal call
			infraReady := true
			// Pre-add finalizer so reconcileNormal skips the early return at line 195
			controllerutil.AddFinalizer(libvMachine, infrastructurev1alpha1.LibvirtMachineFinalizer)
			scope := &MachineScope{
				ReconcilerClient: k8sClient,
				Cluster: &clusterv1.Cluster{
					Status: clusterv1.ClusterStatus{
						Initialization: clusterv1.ClusterInitializationStatus{
							InfrastructureProvisioned: &infraReady,
						},
					},
				},
				Machine: &clusterv1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace,
					},
					Spec: clusterv1.MachineSpec{
						Bootstrap: clusterv1.Bootstrap{
							DataSecretName: &userDataSecret.Name,
						},
					},
				},
				LibvirtCluster: libvCluster,
				LibvirtMachine: libvMachine,
				Ctx:            ctx,
				MachineConfig:  newMachineConfig(libvMachine, libvCluster),
			}

			// Create reconciler with mock provider
			reconciler := &LibvirtMachineReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Provider: &MockProvider{},
			}
			mockProvider := reconciler.Provider.(*MockProvider)
			mockProvider.SetMachineState(machineState)
			mockProvider.SetGetMachineStateErr(mockErr)

			// Call reconcileNormal directly (bypasses owner-check in Reconcile entry point)
			result, err := reconciler.reconcileNormal(scope)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeZero())

			// Verify status was libvMachine
			Expect(libvMachine.Status.Ready).To(Equal(expectReady), "status.ready mismatch")
			Expect(libvMachine.Status.Initialization.Provisioned).To(Equal(expectProvisioned), "status.initialization.provisioned mismatch")

			// Verify condition was set appropriately
			if expectReady && machineState == libvirtclient.DomainStateRunning {
				Expect(libvMachine.Status.Conditions).NotTo(BeEmpty())
				found := false
				for _, c := range libvMachine.Status.Conditions {
					if c.Type == infrastructurev1alpha1.DomainRunningCondition {
						Expect(c.Status).To(Equal(metav1.ConditionTrue))
						found = true
					}
				}
				Expect(found).To(BeTrue(), "DomainRunning condition should be True when machine is running")
			}
		},
		Entry("domain running -> ready and provisioned", libvirtclient.DomainStateRunning, nil, true, true),
		Entry("domain stopped -> started -> ready -> provisioned", libvirtclient.DomainStateStopped, nil, true, true),
		Entry("domain not found -> ready and provisioned after creation", libvirtclient.DomainStateNotFound, nil, true, true),
	)

	It("should handle deletion correctly", func() {
		By("calling reconcileDelete")
		libvMachine := &infrastructurev1alpha1.LibvirtMachine{}
		Expect(k8sClient.Get(ctx, namespacedName, libvMachine)).To(Succeed())
		// Add finalizer — reconcileDelete expects it to be present
		controllerutil.AddFinalizer(libvMachine, infrastructurev1alpha1.LibvirtMachineFinalizer)
		Expect(k8sClient.Update(ctx, libvMachine)).To(Succeed())

		// Get the LibvirtCluster
		libvCluster := &infrastructurev1alpha1.LibvirtCluster{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: testClusterName + "-infra", Namespace: testNamespace}, libvCluster)).To(Succeed())

		// Construct a MachineScope for direct reconcileDelete call
		scope := &MachineScope{
			Cluster:        &clusterv1.Cluster{},
			Machine:        &clusterv1.Machine{},
			LibvirtCluster: libvCluster,
			LibvirtMachine: libvMachine,
			Ctx:            ctx,
			MachineConfig:  newMachineConfig(libvMachine, libvCluster),
		}

		// Create reconciler with mock provider
		reconciler := &LibvirtMachineReconciler{
			Client:   k8sClient,
			Scheme:   k8sClient.Scheme(),
			Provider: &MockProvider{},
		}

		// Call reconcileDelete directly (bypasses owner-check in Reconcile entry point)
		result, err := reconciler.reconcileDelete(scope)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(BeZero())
	})

	Describe("machine address observation", func() {
		var mockProvider *MockProvider

		buildRunningScope := func() *MachineScope {
			libvMachine := &infrastructurev1alpha1.LibvirtMachine{}
			Expect(k8sClient.Get(ctx, namespacedName, libvMachine)).To(Succeed())
			controllerutil.AddFinalizer(libvMachine, infrastructurev1alpha1.LibvirtMachineFinalizer)

			libvCluster := &infrastructurev1alpha1.LibvirtCluster{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: testClusterName + "-infra", Namespace: testNamespace}, libvCluster)).To(Succeed())

			infraReady := true

			return &MachineScope{
				Cluster: &clusterv1.Cluster{
					Status: clusterv1.ClusterStatus{
						Initialization: clusterv1.ClusterInitializationStatus{
							InfrastructureProvisioned: &infraReady,
						},
					},
				},
				Machine:        &clusterv1.Machine{},
				LibvirtCluster: libvCluster,
				LibvirtMachine: libvMachine,
				Ctx:            ctx,
				MachineConfig:  newMachineConfig(libvMachine, libvCluster),
			}
		}

		BeforeEach(func() {
			mockProvider = &MockProvider{}
			mockProvider.SetMachineState(libvirtclient.DomainStateRunning)
		})

		It("populates Status.Addresses from the provider address", func() {
			mockProvider.SetMachineAddress("192.168.122.55")

			reconciler := &LibvirtMachineReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Provider: mockProvider,
			}

			scope := buildRunningScope()
			result, err := reconciler.reconcileNormal(scope)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeZero())

			Expect(scope.LibvirtMachine.Status.Ready).To(BeTrue())
			Expect(scope.LibvirtMachine.Status.Addresses).To(HaveLen(1))
			Expect(scope.LibvirtMachine.Status.Addresses[0].Type).To(Equal(clusterv1.MachineInternalIP))
			Expect(scope.LibvirtMachine.Status.Addresses[0].Address).To(Equal("192.168.122.55"))
		})

		It("requeues when the address lookup fails", func() {
			mockProvider.SetGetMachineAddressErr(fmt.Errorf("get DHCP leases: connection reset"))

			reconciler := &LibvirtMachineReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Provider: mockProvider,
			}

			scope := buildRunningScope()
			result, err := reconciler.reconcileNormal(scope)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).ToNot(BeZero())

			Expect(scope.LibvirtMachine.Status.Ready).To(BeTrue())
			Expect(scope.LibvirtMachine.Status.Addresses).To(BeEmpty())
		})
	})
})
