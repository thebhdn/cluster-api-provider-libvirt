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

package libvirtclient

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/thebhdn/cluster-api-provider-libvirt/internal/libvirtclient/builders"
	libvirt "libvirt.org/go/libvirt"
)

var _ = Describe("isVolumeNotFound", func() {
	When("error is a libvirt ERR_NO_STORAGE_VOL", func() {
		It("returns true", func() {
			err := &libvirt.Error{Code: libvirt.ERR_NO_STORAGE_VOL}
			Expect(isVolumeNotFound(fmt.Errorf("%w", err))).To(BeTrue())
		})

		It("returns true even with domain and message", func() {
			err := &libvirt.Error{
				Code:    libvirt.ERR_NO_STORAGE_VOL,
				Domain:  libvirt.FROM_STORAGE,
				Message: "storage volume not found",
			}
			Expect(isVolumeNotFound(fmt.Errorf("%w", err))).To(BeTrue())
		})
	})

	When("error is a standard error", func() {
		It("returns false", func() {
			Expect(isVolumeNotFound(fmt.Errorf("some other error"))).To(BeFalse())
		})
	})

	When("error is nil", func() {
		It("returns false", func() {
			Expect(isVolumeNotFound(nil)).To(BeFalse())
		})
	})
})

var _ = Describe("volume builder marshaling", func() {
	When("creating a volume builder", func() {
		It("produces valid XML", func() {
			volumeXML, err := builders.NewVolume("test-volume").
				WithCapacity(20, "G").
				WithBackingStore("/var/lib/libvirt/images/base.qcow2", "qcow2").
				Marshal()
			Expect(err).To(BeNil())
			Expect(volumeXML).NotTo(BeEmpty())
		})
	})

	When("creating an ISO disk builder", func() {
		It("produces valid XML", func() {
			volumeXML, err := builders.NewVolume("cloudinit-test.iso").
				WithCapacity(1024, "bytes").
				WithRawType().
				Marshal()
			Expect(err).To(BeNil())
			Expect(volumeXML).NotTo(BeEmpty())
		})
	})

	When("creating a storage volume upload builder", func() {
		It("produces valid XML", func() {
			volumeXML, err := builders.NewVolume("cloudinit.iso").
				WithCapacity(4096, "bytes").
				WithRawType().
				Marshal()
			Expect(err).To(BeNil())
			Expect(volumeXML).NotTo(BeEmpty())
		})
	})
})
