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

package builders

import (
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	libvirtxml "libvirt.org/go/libvirtxml"
)

var _ = ginkgo.Describe("DomainBuilder", func() {
	ginkgo.Context("NewDomain", func() {
		ginkgo.It("creates a domain with the specified name and default settings", func() {
			domain := NewDomain("test-domain")

			gomega.Expect(domain.domain.Type).To(gomega.Equal(DefaultDomainType))
			gomega.Expect(domain.domain.Name).To(gomega.Equal("test-domain"))
			gomega.Expect(domain.domain.Memory.Value).To(gomega.Equal(DefaultMemoryMiB))
			gomega.Expect(domain.domain.VCPU.Value).To(gomega.Equal(DefaultCPU))
			gomega.Expect(domain.domain.OS.Type.Type).To(gomega.Equal(DefaultOSType))
			gomega.Expect(domain.domain.OS.Type.Arch).To(gomega.Equal(DefaultOSArch))
		})

		ginkgo.It("initializes a boot device", func() {
			domain := NewDomain("test-domain")

			gomega.Expect(domain.domain.OS.BootDevices).To(gomega.HaveLen(1))
			gomega.Expect(domain.domain.OS.BootDevices[0].Dev).To(gomega.Equal(DefaultBootDevHD))
		})

		ginkgo.It("initializes Devices and its sub-slices", func() {
			domain := NewDomain("test-domain")

			gomega.Expect(domain.domain.Devices).NotTo(gomega.BeNil())
			gomega.Expect(domain.domain.Devices.Disks).To(gomega.BeEmpty())
			gomega.Expect(domain.domain.Devices.Interfaces).To(gomega.BeEmpty())
		})
	})

	ginkgo.Context("WithMemoryMiB", func() {
		ginkgo.It("sets memory and unit", func() {
			domain := NewDomain("test-domain").WithMemoryMiB(2048)

			gomega.Expect(domain.domain.Memory.Value).To(gomega.Equal(uint(2048)))
			gomega.Expect(domain.domain.Memory.Unit).To(gomega.Equal(DefaultMemoryUnit))
		})
	})

	ginkgo.Context("WithVCPU", func() {
		ginkgo.It("sets vcpu count", func() {
			domain := NewDomain("test-domain").WithVCPU(4)

			gomega.Expect(domain.domain.VCPU.Value).To(gomega.Equal(uint(4)))
		})
	})

	ginkgo.Context("WithDiskFile", func() {
		ginkgo.It("adds a disk with the specified file", func() {
			domain := NewDomain("test-domain").WithDiskFile("/path/to/disk.qcow2")

			gomega.Expect(domain.domain.Devices.Disks).To(gomega.HaveLen(1))
			disk := domain.domain.Devices.Disks[0]
			gomega.Expect(disk.Device).To(gomega.Equal("disk"))
			gomega.Expect(disk.Source.File.File).To(gomega.Equal("/path/to/disk.qcow2"))
			gomega.Expect(disk.Target.Bus).To(gomega.Equal(DefaultDiskBus))
			gomega.Expect(disk.Target.Dev).To(gomega.Equal(DefaultDiskTarget))
		})
	})

	ginkgo.Context("WithCloudInitISO", func() {
		ginkgo.It("adds a cdrom disk pointing to the ISO", func() {
			domain := NewDomain("test-domain").WithCloudInitISO("/path/to/cloud-init.iso")

			gomega.Expect(domain.domain.Devices.Disks).To(gomega.HaveLen(1))
			disk := domain.domain.Devices.Disks[0]
			gomega.Expect(disk.Device).To(gomega.Equal("cdrom"))
			gomega.Expect(disk.Source.File.File).To(gomega.Equal("/path/to/cloud-init.iso"))
			gomega.Expect(disk.ReadOnly).NotTo(gomega.BeNil())
		})
	})

	ginkgo.Context("WithNetwork", func() {
		ginkgo.It("adds a network interface", func() {
			domain := NewDomain("test-domain").WithNetwork("custom-net")

			gomega.Expect(domain.domain.Devices.Interfaces).To(gomega.HaveLen(1))
			iface := domain.domain.Devices.Interfaces[0]
			gomega.Expect(iface.Source.Network.Network).To(gomega.Equal("custom-net"))
			gomega.Expect(iface.Model.Type).To(gomega.Equal(DefaultNetworkModel))
		})

		ginkgo.It("uses default network name when empty", func() {
			domain := NewDomain("test-domain").WithNetwork("")

			gomega.Expect(domain.domain.Devices.Interfaces).To(gomega.HaveLen(1))
			gomega.Expect(
				domain.domain.Devices.Interfaces[0].Source.Network.Network,
			).To(gomega.Equal(DefaultNetworkName))
		})
	})

	ginkgo.Context("WithSerialConsole", func() {
		ginkgo.It("adds a serial port and console device", func() {
			domain := NewDomain("test-domain").WithSerialConsole()

			gomega.Expect(domain.domain.Devices.Serials).To(gomega.HaveLen(1))
			gomega.Expect(domain.domain.Devices.Consoles).To(gomega.HaveLen(1))
			gomega.Expect(*domain.domain.Devices.Serials[0].Target.Port).To(gomega.Equal(uint(0)))
			gomega.Expect(domain.domain.Devices.Consoles[0].Target.Type).To(gomega.Equal("serial"))
		})
	})

	ginkgo.Context("Build", func() {
		ginkgo.It("returns a fully configured Domain after a chain", func() {
			domain := NewDomain("build-test").
				WithMemoryMiB(4096).
				WithVCPU(2).
				WithDiskFile("/path/disk.qcow2").
				WithNetwork("default").
				WithSerialConsole()

			result := domain.Build()

			gomega.Expect(result.Name).To(gomega.Equal("build-test"))
			gomega.Expect(result.Memory.Value).To(gomega.Equal(uint(4096)))
			gomega.Expect(result.VCPU.Value).To(gomega.Equal(uint(2)))
		})
	})

	ginkgo.Context("Build return type", func() {
		ginkgo.It("returns *libvirtxml.Domain", func() {
			domain := NewDomain("type-test")
			result := domain.Build()

			gomega.Expect(result).To(gomega.BeAssignableToTypeOf((*libvirtxml.Domain)(nil)))
		})
	})

	ginkgo.Context("Marshal", func() {
		ginkgo.It("produces valid XML", func() {
			domain := NewDomain("marshal-test").
				WithMemoryMiB(1024).
				WithVCPU(1).
				WithDiskFile("/path/disk.qcow2").
				WithNetwork("default")

			xml, err := domain.Marshal()
			gomega.Expect(err).To(gomega.BeNil())
			gomega.Expect(xml).To(gomega.ContainSubstring("<domain"))
			gomega.Expect(xml).To(gomega.ContainSubstring("marshal-test"))
		})
	})

	ginkgo.Context("Full builder chain", func() {
		ginkgo.It("configures a complete domain with all options", func() {
			domain := NewDomain("full-test").
				WithMemoryMiB(2048).
				WithVCPU(2).
				WithDiskFile("/var/lib/libvirt/images/disk.qcow2").
				WithCloudInitISO("/var/lib/libvirt/images/cloud-init.iso").
				WithNetwork("default").
				WithSerialConsole()

			result := domain.Build()

			gomega.Expect(result.Name).To(gomega.Equal("full-test"))
			gomega.Expect(result.Memory.Value).To(gomega.Equal(uint(2048)))
			gomega.Expect(result.VCPU.Value).To(gomega.Equal(uint(2)))
			gomega.Expect(result.Devices.Disks).To(gomega.HaveLen(2))
			gomega.Expect(result.Devices.Disks[0].Device).To(gomega.Equal("disk"))
			gomega.Expect(result.Devices.Disks[1].Device).To(gomega.Equal("cdrom"))
			gomega.Expect(result.Devices.Interfaces).To(gomega.HaveLen(1))
			gomega.Expect(result.Devices.Serials).To(gomega.HaveLen(1))
		})
	})

	ginkgo.It("marshals all configured fields into XML", func() {
		domain := NewDomain("complete-test").
			WithMemoryMiB(4096).
			WithVCPU(4).
			WithDiskFile("/path/disk.qcow2").
			WithCloudInitISO("/path/cloud-init.iso").
			WithNetwork("mynetwork").
			WithSerialConsole()

		xml, err := domain.Marshal()
		gomega.Expect(err).To(gomega.BeNil())
		gomega.Expect(xml).To(gomega.ContainSubstring("complete-test"))
		gomega.Expect(xml).To(gomega.ContainSubstring("/path/disk.qcow2"))
		gomega.Expect(xml).To(gomega.ContainSubstring("/path/cloud-init.iso"))
		gomega.Expect(xml).To(gomega.ContainSubstring("mynetwork"))
	})
})
