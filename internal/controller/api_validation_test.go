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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrastructurev1alpha1 "github.com/thebhdn/cluster-api-provider-libvirt/api/v1alpha1"
)

const (
	testLibvirtURI        = "qemu+tcp://localhost/system"
	testLibvirtNetwork    = "default"
	testLibvirtBasePool   = "capi-base-images"
	testLibvirtDomainPool = "capi-vm-disks"
	testLibvirtImage      = "base.qcow2"

	validationNamespace = "default"
)

// newTestLibvirtClusterSpec returns a LibvirtClusterSpec that passes CRD validation.
func newTestLibvirtClusterSpec() infrastructurev1alpha1.LibvirtClusterSpec {
	return infrastructurev1alpha1.LibvirtClusterSpec{
		URI:        testLibvirtURI,
		Network:    testLibvirtNetwork,
		BasePool:   testLibvirtBasePool,
		DomainPool: testLibvirtDomainPool,
	}
}

var _ = Describe("API validation", func() {
	newObjectMeta := func(prefix string) metav1.ObjectMeta {
		return metav1.ObjectMeta{GenerateName: prefix, Namespace: validationNamespace}
	}

	deleteAfterTest := func(obj client.Object) {
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, obj))).To(Succeed())
		})
	}

	Describe("LibvirtCluster", func() {
		It("accepts a complete spec", func() {
			cluster := &infrastructurev1alpha1.LibvirtCluster{
				ObjectMeta: newObjectMeta("valid-"),
				Spec:       newTestLibvirtClusterSpec(),
			}
			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())
			deleteAfterTest(cluster)
		})

		DescribeTable("rejects an empty required field",
			func(clear func(*infrastructurev1alpha1.LibvirtClusterSpec)) {
				spec := newTestLibvirtClusterSpec()
				clear(&spec)
				cluster := &infrastructurev1alpha1.LibvirtCluster{
					ObjectMeta: newObjectMeta("invalid-"),
					Spec:       spec,
				}
				Expect(apierrors.IsInvalid(k8sClient.Create(ctx, cluster))).To(BeTrue())
			},
			Entry("uri", func(s *infrastructurev1alpha1.LibvirtClusterSpec) { s.URI = "" }),
			Entry("network", func(s *infrastructurev1alpha1.LibvirtClusterSpec) { s.Network = "" }),
			Entry("basePool", func(s *infrastructurev1alpha1.LibvirtClusterSpec) { s.BasePool = "" }),
			Entry("domainPool", func(s *infrastructurev1alpha1.LibvirtClusterSpec) { s.DomainPool = "" }),
		)
	})

	Describe("LibvirtMachine", func() {
		newMachine := func() *infrastructurev1alpha1.LibvirtMachine {
			return &infrastructurev1alpha1.LibvirtMachine{
				ObjectMeta: newObjectMeta("machine-"),
				Spec:       infrastructurev1alpha1.LibvirtMachineSpec{Image: testLibvirtImage},
			}
		}

		It("applies defaults for vcpu, memory and disk", func() {
			machine := newMachine()
			Expect(k8sClient.Create(ctx, machine)).To(Succeed())
			deleteAfterTest(machine)

			Expect(machine.Spec.VCPU).To(BeEquivalentTo(1))
			Expect(machine.Spec.MemoryMiB).To(BeEquivalentTo(1024))
			Expect(machine.Spec.DiskGiB).To(BeEquivalentTo(20))
		})

		DescribeTable("rejects an invalid spec",
			func(change func(*infrastructurev1alpha1.LibvirtMachineSpec)) {
				machine := newMachine()
				change(&machine.Spec)
				Expect(apierrors.IsInvalid(k8sClient.Create(ctx, machine))).To(BeTrue())
			},
			Entry("empty image", func(s *infrastructurev1alpha1.LibvirtMachineSpec) { s.Image = "" }),
			Entry("negative vcpu", func(s *infrastructurev1alpha1.LibvirtMachineSpec) { s.VCPU = -1 }),
			Entry("memory below minimum", func(s *infrastructurev1alpha1.LibvirtMachineSpec) { s.MemoryMiB = 128 }),
			Entry("negative disk", func(s *infrastructurev1alpha1.LibvirtMachineSpec) { s.DiskGiB = -1 }),
		)

		DescribeTable("rejects a change to an existing machine",
			func(change func(*infrastructurev1alpha1.LibvirtMachineSpec), message string) {
				machine := newMachine()
				Expect(k8sClient.Create(ctx, machine)).To(Succeed())
				deleteAfterTest(machine)

				change(&machine.Spec)
				err := k8sClient.Update(ctx, machine)
				Expect(apierrors.IsInvalid(err)).To(BeTrue())
				Expect(err.Error()).To(ContainSubstring(message))
			},
			Entry("vcpu", func(s *infrastructurev1alpha1.LibvirtMachineSpec) { s.VCPU = 4 }, "vcpu is immutable"),
			Entry("memoryMiB", func(s *infrastructurev1alpha1.LibvirtMachineSpec) { s.MemoryMiB = 4096 }, "memoryMiB is immutable"),
			Entry("diskGiB", func(s *infrastructurev1alpha1.LibvirtMachineSpec) { s.DiskGiB = 40 }, "diskGiB is immutable"),
			Entry("image", func(s *infrastructurev1alpha1.LibvirtMachineSpec) { s.Image = "other.qcow2" }, "image is immutable"),
		)

		It("allows setting the provider ID once", func() {
			machine := newMachine()
			Expect(k8sClient.Create(ctx, machine)).To(Succeed())
			deleteAfterTest(machine)

			setProviderID(machine)
			Expect(k8sClient.Update(ctx, machine)).To(Succeed())

			otherID := providerIDPrefix + "other"
			machine.Spec.ProviderID = &otherID
			err := k8sClient.Update(ctx, machine)
			Expect(apierrors.IsInvalid(err)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("providerID is immutable once set"))
		})
	})

	Describe("LibvirtMachineTemplate", func() {
		It("rejects a change to the machine spec", func() {
			template := &infrastructurev1alpha1.LibvirtMachineTemplate{
				ObjectMeta: newObjectMeta("template-"),
				Spec: infrastructurev1alpha1.LibvirtMachineTemplateSpec{
					Template: infrastructurev1alpha1.LibvirtMachineTemplateResource{
						Spec: infrastructurev1alpha1.LibvirtMachineSpec{Image: testLibvirtImage},
					},
				},
			}
			Expect(k8sClient.Create(ctx, template)).To(Succeed())
			deleteAfterTest(template)

			template.Spec.Template.Spec.VCPU = 4
			err := k8sClient.Update(ctx, template)
			Expect(apierrors.IsInvalid(err)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("vcpu is immutable"))
		})
	})
})
