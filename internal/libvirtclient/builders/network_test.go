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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	libvirtxml "libvirt.org/go/libvirtxml"
)

var _ = Describe("NetworkBuilder", func() {
	Context("NewNetwork", func() {
		It("creates a network with name, NAT forward mode and gateway IP", func() {
			network, err := NewNetwork("test-net", "192.168.112.0/24")
			Expect(err).To(BeNil())

			Expect(network.network.Name).To(Equal("test-net"))
			Expect(network.network.Forward).NotTo(BeNil())
			Expect(network.network.Forward.Mode).To(Equal(DefaultNetworkForwardMode))
			Expect(network.network.IPs).To(HaveLen(1))
			Expect(network.network.IPs[0].Address).To(Equal("192.168.112.1"))
			Expect(network.network.IPs[0].Family).To(Equal(DefaultNetworkFamily))
			Expect(network.network.IPs[0].Prefix).To(Equal(uint(24)))
		})

		It("defaults to no bridge when unset", func() {
			network, err := NewNetwork("no-bridge", "10.0.0.0/16")
			Expect(err).To(BeNil())

			Expect(network.network.Bridge).To(BeNil())
		})

		It("derives the gateway from the masked CIDR", func() {
			cases := []struct {
				cidr    string
				gateway string
			}{
				{"10.10.0.0/16", "10.10.0.1"},
				{"172.16.5.0/24", "172.16.5.1"},
				{"192.168.1.33/24", "192.168.1.1"},
			}

			for _, tc := range cases {
				network, err := NewNetwork("gateway-test", tc.cidr)
				Expect(err).To(BeNil())
				Expect(network.network.IPs[0].Address).To(Equal(tc.gateway))
			}
		})

		It("returns an error for an invalid CIDR", func() {
			_, err := NewNetwork("bad-net", "not-a-cidr")
			Expect(err).NotTo(BeNil())
		})
	})

	Context("WithBridge", func() {
		It("sets the bridge name", func() {
			network, err := NewNetwork("bridge-net", "192.168.1.0/24")
			Expect(err).To(BeNil())
			network = network.WithBridge("br-test")

			Expect(network.network.Bridge).NotTo(BeNil())
			Expect(network.network.Bridge.Name).To(Equal("br-test"))
		})
	})

	Context("WithDHCPRange", func() {
		It("sets the DHCP range on the network IP", func() {
			network, err := NewNetwork("dhcp-net", "192.168.112.0/24")
			Expect(err).To(BeNil())
			network = network.WithDHCPRange("192.168.112.10", "192.168.112.250")

			Expect(network.network.IPs[0].DHCP).NotTo(BeNil())
			Expect(network.network.IPs[0].DHCP.Ranges).To(HaveLen(1))
			Expect(network.network.IPs[0].DHCP.Ranges[0].Start).To(Equal("192.168.112.10"))
			Expect(network.network.IPs[0].DHCP.Ranges[0].End).To(Equal("192.168.112.250"))
		})

		It("replaces the range when called again", func() {
			network, err := NewNetwork("dhcp-replace", "192.168.112.0/24")
			Expect(err).To(BeNil())
			network = network.
				WithDHCPRange("192.168.112.10", "192.168.112.250").
				WithDHCPRange("192.168.112.50", "192.168.112.200")

			Expect(network.network.IPs[0].DHCP.Ranges).To(HaveLen(1))
			Expect(network.network.IPs[0].DHCP.Ranges[0].Start).To(Equal("192.168.112.50"))
			Expect(network.network.IPs[0].DHCP.Ranges[0].End).To(Equal("192.168.112.200"))
		})
	})

	Context("Build", func() {
		It("returns a fully configured Network after a chain", func() {
			network, err := NewNetwork("build-test", "192.168.112.0/24")
			Expect(err).To(BeNil())
			network = network.
				WithBridge("br-test").
				WithDHCPRange("192.168.112.10", "192.168.112.250")

			result := network.Build()

			Expect(result.Name).To(Equal("build-test"))
			Expect(result.Forward.Mode).To(Equal(DefaultNetworkForwardMode))
			Expect(result.Bridge.Name).To(Equal("br-test"))
			Expect(result.IPs[0].DHCP.Ranges).To(HaveLen(1))
		})
	})

	Context("Build return type", func() {
		It("returns *libvirtxml.Network", func() {
			network, err := NewNetwork("type-test", "10.0.0.0/8")
			Expect(err).To(BeNil())
			result := network.Build()

			Expect(result).To(BeAssignableToTypeOf((*libvirtxml.Network)(nil)))
		})
	})

	Context("Marshal", func() {
		It("produces valid XML with all configured fields", func() {
			network, err := NewNetwork("marshal-test", "192.168.112.0/24")
			Expect(err).To(BeNil())
			network = network.
				WithBridge("br-test").
				WithDHCPRange("192.168.112.10", "192.168.112.250")

			xml, err := network.Marshal()
			Expect(err).To(BeNil())
			Expect(xml).To(ContainSubstring("<network>"))
			Expect(xml).To(ContainSubstring("marshal-test"))
			Expect(xml).To(ContainSubstring(`mode="nat"`))
			Expect(xml).To(ContainSubstring(`<bridge name="br-test"`))
			Expect(xml).To(ContainSubstring("192.168.112.1"))
			Expect(xml).To(ContainSubstring(`family="inet"`))
			Expect(xml).To(ContainSubstring(`prefix="24"`))
			Expect(xml).To(ContainSubstring(`start="192.168.112.10"`))
			Expect(xml).To(ContainSubstring(`end="192.168.112.250"`))
		})
	})

	It("marshals a complete network configuration", func() {
		network, err := NewNetwork("complete-test", "10.10.0.0/16")
		Expect(err).To(BeNil())
		network = network.
			WithBridge("br10").
			WithDHCPRange("10.10.0.10", "10.10.254.250")

		xml, err := network.Marshal()
		Expect(err).To(BeNil())
		Expect(xml).To(ContainSubstring("complete-test"))
		Expect(xml).To(ContainSubstring("10.10.0.1"))
		Expect(xml).To(ContainSubstring("br10"))
	})
})
