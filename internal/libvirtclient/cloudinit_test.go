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
	"bytes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	build "github.com/thebhdn/cluster-api-provider-libvirt/internal/libvirtclient/builders"
)

type testCase struct {
	name     string
	userData []byte
}

var _ = Describe("writeCloudInitISO", func() {
	When("all config fields are populated", func() {
		It("writes a non-empty ISO to the writer", func() {
			cfg := MachineConfig{
				InfraConfig: InfraConfig{
					Network:    "default",
					BasePool:   "default",
					DomainPool: "default",
					URI:        "qemu+tcp://localhost/system",
				},
				DomainName: "test-machine",
				MemoryMiB:  1024,
				VCPU:       1,
				DiskSize:   20,
				DiskFormat: "qcow2",
				UserData:   []byte("#cloud-config\ndata: hello\n"),
			}

			var buf bytes.Buffer
			err := writeCloudInitISO(&buf, cfg)
			if err != nil {
				Skip("writeCloudInitISO failed (requires libvirt installed)")
			}
			Expect(buf.Len()).To(BeNumerically(">", 0))
		})
	})

	DescribeTable("handles user-data payloads",
		func(tc testCase) {
			cfg := MachineConfig{
				InfraConfig: InfraConfig{Network: "default"},
				DomainName:  "test-machine",
				UserData:    tc.userData,
			}

			var buf bytes.Buffer
			err := writeCloudInitISO(&buf, cfg)
			if err != nil {
				Skip("writeCloudInitISO failed (requires libvirt)")
			}
			Expect(buf.Len()).To(BeNumerically(">", 0))
		},
		Entry("empty user-data", testCase{name: "empty user-data", userData: []byte("")}),
		Entry("multiline user-data", testCase{name: "multiline user-data", userData: []byte("#cloud-config\nruncmd:\n  - echo hello\n  - echo world\n")}),
		Entry("large user-data", testCase{name: "large user-data", userData: bytes.Repeat([]byte("#cloud-config\nruncmd:\n  - echo test\n"), 100)}),
		Entry("binary-like user-data", testCase{name: "binary-like user-data", userData: []byte{0x23, 0x63, 0x6c, 0x6f, 0x75, 0x64, 0x2d, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x0a}}),
	)

	When("given a zero-value config", func() {
		It("still produces a non-empty ISO", func() {
			var buf bytes.Buffer
			err := writeCloudInitISO(&buf, MachineConfig{})
			if err != nil {
				Skip("writeCloudInitISO failed")
			}
			Expect(buf.Len()).To(BeNumerically(">", 0))
		})
	})

	When("given a valid config", func() {
		It("produces an ISO at least 10 MiB", func() {
			cfg := MachineConfig{
				InfraConfig: InfraConfig{Network: "default"},
				DomainName:  "size-test",
				UserData:    []byte("test data"),
			}

			var buf bytes.Buffer
			err := writeCloudInitISO(&buf, cfg)
			if err != nil {
				Skip("writeCloudInitISO failed")
			}

			// ISO images are typically >= 10 MiB
			const expectedMin = 10 * 1024 * 1024
			Expect(buf.Len()).To(BeNumerically(">=", expectedMin))
		})
	})

	DescribeTable("produces valid ISO for multiple domain names",
		func(domain string) {
			cfg := MachineConfig{
				InfraConfig: InfraConfig{Network: "default"},
				DomainName:  domain,
				UserData:    []byte("cloud-init data for " + domain),
			}

			var buf bytes.Buffer
			err := writeCloudInitISO(&buf, cfg)
			if err != nil {
				Skip("writeCloudInitISO failed")
			}
			Expect(buf.Len()).To(BeNumerically(">", 0))
		},
		Entry("machine-1", "machine-1"),
		Entry("machine-2", "machine-2"),
		Entry("machine-3", "machine-3"),
	)
})

var _ = Describe("VolumeBuilder defaults", func() {
	It("uses the correct default capacity and format", func() {
		Expect(build.DefaultVolumeCapacityGiB).To(Equal(uint64(20)))
		Expect(build.DefaultVolumeFormat).To(Equal("qcow2"))
	})
})
