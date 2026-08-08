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
)

var _ = Describe("NewProvider", func() {
	It("returns a non-nil Provider", func() {
		provider := NewProvider()
		Expect(provider).NotTo(BeNil())
	})

	It("nil check works", func() {
		var p *Provider
		Expect(p).To(BeNil())

		p = NewProvider()
		Expect(p).NotTo(BeNil())
	})
})

var _ = Describe("Provider error wrapping", func() {
	When("EnsureInfra is called with an invalid URI", func() {
		It("wraps the connection error", func() {
			provider := NewProvider()
			cfg := InfraConfig{
				URI:        "qemu+tcp://invalid-host:9999/system",
				Network:    "default",
				BasePool:   "default",
				DomainPool: "default",
			}

			err := provider.EnsureInfra(cfg)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("connect to libvirt host:"))
		})
	})

	When("CreateMachine is called with an invalid URI", func() {
		It("wraps the connection error", func() {
			provider := NewProvider()
			cfg := MachineConfig{
				InfraConfig: InfraConfig{
					URI:        "qemu+tcp://invalid-host:9999/system",
					Network:    "default",
					BasePool:   "default",
					DomainPool: "default",
				},
				DomainName: "test-machine",
			}

			_, err := provider.CreateMachine(cfg)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("connect to libvirt host:"))
		})
	})

	When("StartMachine is called with an invalid URI", func() {
		It("wraps the connection error", func() {
			provider := NewProvider()
			cfg := MachineConfig{
				InfraConfig: InfraConfig{
					URI: "qemu+tcp://invalid-host:9999/system",
				},
				DomainName: "test-machine",
			}

			err := provider.StartMachine(cfg)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("connect to libvirt host:"))
		})
	})

	When("GetMachineState is called with an invalid URI", func() {
		It("wraps the connection error", func() {
			provider := NewProvider()
			cfg := MachineConfig{
				InfraConfig: InfraConfig{
					URI: "qemu+tcp://invalid-host:9999/system",
				},
				DomainName: "test-machine",
			}

			state, err := provider.GetMachineState(cfg)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("connect to libvirt host:"))
			Expect(state).To(Equal(DomainState("")))
		})
	})

	When("DeleteMachine is called with an invalid URI", func() {
		It("wraps the connection error", func() {
			provider := NewProvider()
			cfg := MachineConfig{
				InfraConfig: InfraConfig{
					URI:        "qemu+tcp://invalid-host:9999/system",
					DomainPool: "default",
				},
				DomainName: "test-machine",
			}

			err := provider.DeleteMachine(cfg)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("connect to libvirt host:"))
		})
	})
})

var _ = Describe("Provider methods with empty config", func() {
	It("EnsureInfra with empty config wraps connection error", func() {
		err := NewProvider().EnsureInfra(InfraConfig{URI: "qemu+tcp://nonexistent:1234/system"})
		Expect(err).NotTo(BeNil())
		Expect(err.Error()).To(ContainSubstring("connect to libvirt host"))
	})

	It("CreateMachine with empty config wraps connection error", func() {
		_, err := NewProvider().CreateMachine(MachineConfig{InfraConfig: InfraConfig{URI: "qemu+tcp://nonexistent:1234/system"}})
		Expect(err).NotTo(BeNil())
		Expect(err.Error()).To(ContainSubstring("connect to libvirt host"))
	})

	It("StartMachine with empty config wraps connection error", func() {
		err := NewProvider().StartMachine(MachineConfig{InfraConfig: InfraConfig{URI: "qemu+tcp://nonexistent:1234/system"}})
		Expect(err).NotTo(BeNil())
		Expect(err.Error()).To(ContainSubstring("connect to libvirt host"))
	})

	It("DeleteMachine with empty config wraps connection error", func() {
		err := NewProvider().DeleteMachine(MachineConfig{InfraConfig: InfraConfig{URI: "qemu+tcp://nonexistent:1234/system"}})
		Expect(err).NotTo(BeNil())
		Expect(err.Error()).To(ContainSubstring("connect to libvirt host"))
	})
})

var _ = Describe("MachineConfig userdata", func() {
	It("preserves the userdata byte slice", func() {
		userData := []byte("#cloud-config\nruncmd:\n  - echo hello\n")
		cfg := MachineConfig{UserData: userData}

		Expect(cfg.UserData).To(HaveExactElements(
			[]byte("#cloud-config\nruncmd:\n  - echo hello\n"),
		))
	})
})
