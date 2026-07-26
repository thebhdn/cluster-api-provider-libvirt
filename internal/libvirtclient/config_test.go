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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	build "github.com/thebhdn/cluster-api-provider-libvirt/internal/libvirtclient/builders"
)

var _ = Describe("MachineConfig accessors", func() {
	When("domainName is set", func() {
		It("returns the configured name", func() {
			cfg := MachineConfig{DomainName: "test-machine"}
			Expect(cfg.domainName()).To(Equal("test-machine"))
		})
	})

	When("domainDiskName is computed", func() {
		It("appends .qcow2 extension", func() {
			cfg := MachineConfig{DomainName: "test-machine"}
			Expect(cfg.domainDiskName()).To(Equal("test-machine.qcow2"))
		})
	})

	When("isoDiskName is computed", func() {
		It("prepends cloudinit- and appends .iso", func() {
			cfg := MachineConfig{DomainName: "test-machine"}
			Expect(cfg.isoDiskName()).To(Equal("cloudinit-test-machine.iso"))
		})
	})

	When("MemoryMiB is set", func() {
		It("returns the configured value", func() {
			cfg := MachineConfig{MemoryMiB: 4096}
			Expect(cfg.memoryMiB()).To(Equal(uint(4096)))
		})

		It("falls back to builder default when zero", func() {
			cfg := MachineConfig{MemoryMiB: 0}
			Expect(cfg.memoryMiB()).To(Equal(build.DefaultMemoryMiB))
		})
	})

	When("VCPU is set", func() {
		It("returns the configured value", func() {
			cfg := MachineConfig{VCPU: 4}
			Expect(cfg.vCPU()).To(Equal(uint(4)))
		})

		It("falls back to builder default when zero", func() {
			cfg := MachineConfig{VCPU: 0}
			Expect(cfg.vCPU()).To(Equal(build.DefaultCPU))
		})
	})

	When("DiskSize is set", func() {
		It("returns the configured value", func() {
			cfg := MachineConfig{DiskSize: 50}
			Expect(cfg.diskSizeGiB()).To(Equal(uint64(50)))
		})

		It("falls back to builder default when zero", func() {
			cfg := MachineConfig{DiskSize: 0}
			Expect(cfg.diskSizeGiB()).To(Equal(build.DefaultVolumeCapacityGiB))
		})
	})
})

var _ = Describe("InfraConfig accessors", func() {
	When("network is set", func() {
		It("returns the configured network name", func() {
			cfg := InfraConfig{Network: "default"}
			Expect(cfg.networkName()).To(Equal("default"))
		})
	})

	When("base pool is set", func() {
		It("returns the configured base pool name", func() {
			cfg := InfraConfig{BasePool: "base-pool"}
			Expect(cfg.basePoolName()).To(Equal("base-pool"))
		})
	})

	When("domain pool is set", func() {
		It("returns the configured domain pool name", func() {
			cfg := InfraConfig{DomainPool: "domain-pool"}
			Expect(cfg.domainPoolName()).To(Equal("domain-pool"))
		})
	})

	When("URI is set", func() {
		It("returns the configured URI", func() {
			cfg := InfraConfig{URI: "qemu+ssh://user@host/system"}
			Expect(cfg.getURI()).To(Equal("qemu+ssh://user@host/system"))
		})
	})
})

var _ = Describe("MachineConfig embedding", func() {
	It("exposes embedded InfraConfig fields", func() {
		cfg := MachineConfig{
			InfraConfig: InfraConfig{
				URI:        "qemu+tls://host/system",
				BasePool:   "storage",
				DomainPool: "domain-storage",
				Network:    "default",
			},
			DomainName: "test",
		}

		Expect(cfg.URI).To(Equal("qemu+tls://host/system"))
		Expect(cfg.BasePool).To(Equal("storage"))
		Expect(cfg.DomainPool).To(Equal("domain-storage"))
		Expect(cfg.Network).To(Equal("default"))
	})

	It("resolves all accessors with a fully populated config", func() {
		cfg := MachineConfig{
			InfraConfig: InfraConfig{
				URI:        "qemu+ssh://root@192.168.1.100/system",
				BasePool:   "default",
				DomainPool: "images",
				Network:    "virbr0",
			},
			DomainName: "kubelet-node-01",
			BaseImage:  "ubuntu-22.04.qcow2",
			MemoryMiB:  8192,
			VCPU:       4,
			DiskSize:   100,
			DiskFormat: "qcow2",
			UserData:   []byte("cloud-config data"),
		}

		Expect(cfg.domainName()).To(Equal("kubelet-node-01"))
		Expect(cfg.domainDiskName()).To(Equal("kubelet-node-01.qcow2"))
		Expect(cfg.isoDiskName()).To(Equal("cloudinit-kubelet-node-01.iso"))
		Expect(cfg.memoryMiB()).To(Equal(uint(8192)))
		Expect(cfg.vCPU()).To(Equal(uint(4)))
		Expect(cfg.diskSizeGiB()).To(Equal(uint64(100)))
	})
})

var _ = Describe("connect error wrapping", func() {
	It("wraps connection errors with a descriptive prefix", func() {
		_, err := connect("invalid-uri://invalid-host")
		Expect(err).NotTo(BeNil())
		Expect(err.Error()).To(ContainSubstring("connect to libvirt:"))
	})

	It("wraps connection errors even with a valid URI scheme", func() {
		_, err := connect("qemu+tcp://localhost/system")
		if err == nil {
			Skip("libvirt not available; skipping connection error test")
		}
		Expect(err.Error()).To(ContainSubstring("connect to libvirt:"))
	})
})
