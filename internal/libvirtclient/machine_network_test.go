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
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	libvirt "libvirt.org/go/libvirt"
	libvirtxml "libvirt.org/go/libvirtxml"
)

// fakeNetworkOps implements networkOperations with canned XML and leases,
// recording every Update call.
type fakeNetworkOps struct {
	xml         string
	leases      []libvirt.NetworkDHCPLease
	leasesErr   error
	updateErr   error
	updateCalls []fakeUpdateCall
}

type fakeUpdateCall struct {
	cmd     libvirt.NetworkUpdateCommand
	section libvirt.NetworkUpdateSection
	xml     string
}

func (f *fakeNetworkOps) GetXMLDesc(flags libvirt.NetworkXMLFlags) (string, error) {
	return f.xml, nil
}

func (f *fakeNetworkOps) Update(cmd libvirt.NetworkUpdateCommand, section libvirt.NetworkUpdateSection, parentIndex int, xml string, flags libvirt.NetworkUpdateFlags) error {
	f.updateCalls = append(f.updateCalls, fakeUpdateCall{cmd: cmd, section: section, xml: xml})
	return f.updateErr
}

func (f *fakeNetworkOps) GetDHCPLeases() ([]libvirt.NetworkDHCPLease, error) {
	return f.leases, f.leasesErr
}

// testNetworkXML builds a minimal network XML document on 192.168.122.1/24
// unless netmask says otherwise.
func testNetworkXML(netmask, rangeStart, rangeEnd string, hostsXML string) string {
	dhcp := ""
	if rangeStart != "" {
		dhcp = fmt.Sprintf(`<range start="%s" end="%s"/>`, rangeStart, rangeEnd)
	}
	dhcp += hostsXML
	if dhcp != "" {
		dhcp = "<dhcp>" + dhcp + "</dhcp>"
	}
	return fmt.Sprintf(`<network><name>testnet</name><ip address="192.168.122.1" netmask="%s" family="inet">%s</ip></network>`, netmask, dhcp)
}

func testHostEntry(mac, name, ip string) string {
	return fmt.Sprintf(`<host mac="%s" name="%s" ip="%s"/>`, mac, name, ip)
}

var _ = Describe("deriveDomainMAC", func() {
	It("is deterministic per domain name", func() {
		first := deriveDomainMAC("test-machine")
		second := deriveDomainMAC("test-machine")
		Expect(first).To(Equal(second))
	})

	It("uses the libvirt vendor-local prefix", func() {
		Expect(deriveDomainMAC("test-machine")).To(HavePrefix("52:54:00:"))
	})

	It("is locally administered (U/L bit set)", func() {
		mac, err := net.ParseMAC(deriveDomainMAC("test-machine"))
		Expect(err).NotTo(HaveOccurred())
		Expect(mac[0] & 0x02).NotTo(BeZero())
	})

	It("is unique across distinct names", func() {
		seen := make(map[string]bool)
		for i := range 100 {
			name := fmt.Sprintf("machine-%d", i)
			mac := deriveDomainMAC(name)
			seen[mac] = true
		}
		Expect(seen).To(HaveLen(100))
	})
})

var _ = Describe("leaseAddressForMac", func() {
	It("returns the IP of the lease bound to the MAC", func() {
		leases := []libvirt.NetworkDHCPLease{
			{Mac: "aa:bb:cc:dd:ee:00", IPaddr: "192.168.122.10"},
			{Mac: "52:54:00:aa:bb:cc", IPaddr: "192.168.122.55"},
		}
		Expect(leaseAddressForMac(leases, "52:54:00:aa:bb:cc")).To(Equal("192.168.122.55"))
	})

	It("matches case-insensitively", func() {
		leases := []libvirt.NetworkDHCPLease{{Mac: "52:54:00:aa:bb:cc", IPaddr: "192.168.122.55"}}
		Expect(leaseAddressForMac(leases, "52:54:00:AA:BB:CC")).To(Equal("192.168.122.55"))
	})

	It("returns empty when no lease matches", func() {
		leases := []libvirt.NetworkDHCPLease{{Mac: "aa:bb:cc:dd:ee:00", IPaddr: "192.168.122.10"}}
		Expect(leaseAddressForMac(leases, "52:54:00:aa:bb:cc")).To(BeEmpty())
	})

	It("returns empty when there are no leases", func() {
		Expect(leaseAddressForMac(nil, "52:54:00:aa:bb:cc")).To(BeEmpty())
	})
})

var _ = Describe("parseSubnet", func() {
	It("derives the subnet from address and netmask", func() {
		ipNet, err := parseSubnet("192.168.122.1", "255.255.255.0")
		Expect(err).NotTo(HaveOccurred())
		Expect(ipNet.String()).To(Equal("192.168.122.0/24"))
	})

	It("rejects an invalid address", func() {
		_, err := parseSubnet("not-an-ip", "255.255.255.0")
		Expect(err).To(HaveOccurred())
	})

	It("rejects an invalid netmask", func() {
		_, err := parseSubnet("192.168.122.1", "255.255.255.")
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("allocationCandidates", func() {
	var network *libvirtxml.Network

	When("the network defines a DHCP range", func() {
		It("returns exactly the range IPs", func() {
			network = &libvirtxml.Network{}
			Expect(network.Unmarshal(testNetworkXML("255.255.255.0", "192.168.122.10", "192.168.122.19", ""))).To(Succeed())

			candidates, err := allocationCandidates(network)
			Expect(err).NotTo(HaveOccurred())
			Expect(candidates[0]).To(Equal(parseIPNum("192.168.122.10")))
			Expect(candidates[len(candidates)-1]).To(Equal(parseIPNum("192.168.122.19")))
			Expect(candidates).To(HaveLen(10))
		})
	})

	When("the network has no DHCP range", func() {
		It("returns the last machineReservationBlockSize subnet hosts", func() {
			network = &libvirtxml.Network{}
			Expect(network.Unmarshal(testNetworkXML("255.255.255.0", "", "", ""))).To(Succeed())

			candidates, err := allocationCandidates(network)
			Expect(err).NotTo(HaveOccurred())
			Expect(candidates).To(HaveLen(machineReservationBlockSize))
			Expect(candidates[0]).To(Equal(parseIPNum("192.168.122.55")))
			Expect(candidates[len(candidates)-1]).To(Equal(parseIPNum("192.168.122.254")))
		})
	})

	When("the subnet is smaller than the block size", func() {
		It("returns every usable host", func() {
			network = &libvirtxml.Network{}
			Expect(network.Unmarshal(testNetworkXML("255.255.255.252", "", "", ""))).To(Succeed())

			candidates, err := allocationCandidates(network)
			Expect(err).NotTo(HaveOccurred())
			Expect(candidates).To(Equal([]uint32{parseIPNum("192.168.122.1"), parseIPNum("192.168.122.2")}))
		})
	})

	When("the network has no IPv4 subnet", func() {
		It("returns an error", func() {
			network = &libvirtxml.Network{}
			Expect(network.Unmarshal(`<network><ip address="::1" family="ipv6"/></network>`)).To(Succeed())

			_, err := allocationCandidates(network)
			Expect(err).To(HaveOccurred())
		})
	})
})

var _ = Describe("allocateMachineIPIn", func() {
	var network *libvirtxml.Network
	const (
		domainName = "machine-a"
		otherMAC   = "de:ad:be:ef:00:01"
	)

	BeforeEach(func() {
		network = &libvirtxml.Network{}
		Expect(network.Unmarshal(testNetworkXML("255.255.255.0", "", "", ""))).To(Succeed())
	})

	It("is deterministic per domain name", func() {
		mac := deriveDomainMAC(domainName)
		first, err := allocateMachineIPIn(network, domainName, mac)
		Expect(err).NotTo(HaveOccurred())
		second, err := allocateMachineIPIn(network, domainName, mac)
		Expect(err).NotTo(HaveOccurred())
		Expect(first).To(Equal(second))
	})

	When("the candidate at the deterministic offset is taken by another MAC", func() {
		It("moves to the next candidate", func() {
			candidates, err := allocationCandidates(network)
			Expect(err).NotTo(HaveOccurred())
			mac := deriveDomainMAC(domainName)
			offset := int(hashDomainName(domainName) % uint64(len(candidates)))

			takenIP := formatIPv4(candidates[offset])
			hosts := testHostEntry(otherMAC, "other-machine", takenIP)
			refreshed := &libvirtxml.Network{}
			Expect(refreshed.Unmarshal(testNetworkXML("255.255.255.0", "", "", hosts))).To(Succeed())

			ip, err := allocateMachineIPIn(refreshed, domainName, mac)
			Expect(err).NotTo(HaveOccurred())
			Expect(ip).To(Equal(formatIPv4(candidates[(offset+1)%len(candidates)])))
		})
	})

	When("an entry already binds the same MAC", func() {
		It("treats the IP as free", func() {
			mac := deriveDomainMAC(domainName)
			candidates, err := allocationCandidates(network)
			Expect(err).NotTo(HaveOccurred())
			offset := int(hashDomainName(domainName) % uint64(len(candidates)))
			selfIP := formatIPv4(candidates[offset])

			hosts := testHostEntry(mac, domainName, selfIP)
			refreshed := &libvirtxml.Network{}
			Expect(refreshed.Unmarshal(testNetworkXML("255.255.255.0", "", "", hosts))).To(Succeed())

			ip, err := allocateMachineIPIn(refreshed, domainName, mac)
			Expect(err).NotTo(HaveOccurred())
			Expect(ip).To(Equal(selfIP))
		})
	})

	When("every candidate is taken by other MACs", func() {
		It("returns an error", func() {
			small := &libvirtxml.Network{}
			Expect(small.Unmarshal(testNetworkXML("255.255.255.252", "", "",
				testHostEntry("de:ad:be:ef:00:01", "m1", "192.168.122.1")+testHostEntry("de:ad:be:ef:00:02", "m2", "192.168.122.2"),
			))).To(Succeed())

			_, err := allocateMachineIPIn(small, domainName, deriveDomainMAC(domainName))
			Expect(err).To(HaveOccurred())
		})
	})
})

var _ = Describe("machineHostXML", func() {
	It("marshals the host entry attributes", func() {
		xml, err := machineHostXML("machine-a", "52:54:00:aa:bb:cc", "192.168.122.55")
		Expect(err).NotTo(HaveOccurred())
		Expect(xml).To(ContainSubstring(`mac="52:54:00:aa:bb:cc"`))
		Expect(xml).To(ContainSubstring(`name="machine-a"`))
		Expect(xml).To(ContainSubstring(`ip="192.168.122.55"`))
	})
})

var _ = Describe("planHostUpdate", func() {
	const domainName = "machine-a"

	When("there is no host entry for the MAC", func() {
		It("plans ADD_LAST with an allocated IP", func() {
			network := &libvirtxml.Network{}
			Expect(network.Unmarshal(testNetworkXML("255.255.255.0", "", "", ""))).To(Succeed())
			mac := deriveDomainMAC(domainName)

			ip, xml, cmd, doUpdate, err := planHostUpdate(network, domainName, mac)
			Expect(err).NotTo(HaveOccurred())
			Expect(doUpdate).To(BeTrue())
			Expect(cmd).To(Equal(libvirt.NETWORK_UPDATE_COMMAND_ADD_LAST))
			Expect(ip).To(Equal(formatIPv4(parseIPNum(ip))))
			Expect(xml).To(ContainSubstring(mac))
		})
	})

	When("the entry already binds the MAC to the same IP", func() {
		It("is an idempotent no-op", func() {
			mac := deriveDomainMAC(domainName)
			// First allocate to learn the deterministic IP for this name.
			empty := &libvirtxml.Network{}
			Expect(empty.Unmarshal(testNetworkXML("255.255.255.0", "", "", ""))).To(Succeed())
			ip, _, _, _, err := planHostUpdate(empty, domainName, mac)
			Expect(err).NotTo(HaveOccurred())

			network := &libvirtxml.Network{}
			Expect(network.Unmarshal(testNetworkXML("255.255.255.0", "", "", testHostEntry(mac, domainName, ip)))).To(Succeed())

			boundIP, xml, cmd, doUpdate, err := planHostUpdate(network, domainName, mac)
			Expect(err).NotTo(HaveOccurred())
			Expect(doUpdate).To(BeFalse())
			Expect(cmd).To(Equal(libvirt.NETWORK_UPDATE_COMMAND_NONE))
			Expect(xml).To(BeEmpty())
			Expect(boundIP).To(Equal(ip))
		})
	})

	When("the entry binds the MAC to a different IP", func() {
		It("plans MODIFY to re-define the entry", func() {
			mac := deriveDomainMAC(domainName)
			network := &libvirtxml.Network{}
			Expect(network.Unmarshal(testNetworkXML("255.255.255.0", "", "", testHostEntry(mac, domainName, "192.168.122.99")))).To(Succeed())

			_, _, cmd, doUpdate, err := planHostUpdate(network, domainName, mac)
			Expect(err).NotTo(HaveOccurred())
			Expect(doUpdate).To(BeTrue())
			Expect(cmd).To(Equal(libvirt.NETWORK_UPDATE_COMMAND_MODIFY))
		})
	})
})

var _ = Describe("reserveMachineIP", func() {
	const domainName = "machine-a"

	When("there is no host entry", func() {
		It("issues exactly one ADD_LAST update", func() {
			mac := deriveDomainMAC(domainName)
			fake := &fakeNetworkOps{
				xml: testNetworkXML("255.255.255.0", "", "", ""),
			}

			ip, err := reserveMachineIP(fake, "testnet", domainName, mac)
			Expect(err).NotTo(HaveOccurred())
			Expect(ip).NotTo(BeEmpty())
			Expect(fake.updateCalls).To(HaveLen(1))
			Expect(fake.updateCalls[0].cmd).To(Equal(libvirt.NETWORK_UPDATE_COMMAND_ADD_LAST))
			Expect(fake.updateCalls[0].section).To(Equal(libvirt.NETWORK_SECTION_IP_DHCP_HOST))
			Expect(fake.updateCalls[0].xml).To(ContainSubstring(ip))
		})
	})

	When("the entry already exists with the same IP", func() {
		It("is idempotent and issues no update", func() {
			mac := deriveDomainMAC(domainName)
			empty := &libvirtxml.Network{}
			Expect(empty.Unmarshal(testNetworkXML("255.255.255.0", "", "", ""))).To(Succeed())
			ip, _, _, _, err := planHostUpdate(empty, domainName, mac)
			Expect(err).NotTo(HaveOccurred())

			fake := &fakeNetworkOps{
				xml: testNetworkXML("255.255.255.0", "", "", testHostEntry(mac, domainName, ip)),
			}

			boundIP, err := reserveMachineIP(fake, "testnet", domainName, mac)
			Expect(err).NotTo(HaveOccurred())
			Expect(boundIP).To(Equal(ip))
			Expect(fake.updateCalls).To(BeEmpty())
		})
	})
})

var _ = Describe("releaseMachineIP", func() {
	const domainName = "machine-a"

	When("the entry exists", func() {
		It("issues exactly one DELETE update", func() {
			mac := deriveDomainMAC(domainName)
			fake := &fakeNetworkOps{
				xml: testNetworkXML("255.255.255.0", "", "", testHostEntry(mac, domainName, "192.168.122.55")),
			}

			Expect(releaseMachineIP(fake, "testnet", domainName, mac)).To(Succeed())
			Expect(fake.updateCalls).To(HaveLen(1))
			Expect(fake.updateCalls[0].cmd).To(Equal(libvirt.NETWORK_UPDATE_COMMAND_DELETE))
			Expect(fake.updateCalls[0].section).To(Equal(libvirt.NETWORK_SECTION_IP_DHCP_HOST))
		})
	})

	When("the entry is already gone", func() {
		It("succeeds without any update", func() {
			fake := &fakeNetworkOps{
				xml: testNetworkXML("255.255.255.0", "", "", ""),
			}

			Expect(releaseMachineIP(fake, "testnet", domainName, deriveDomainMAC(domainName))).To(Succeed())
			Expect(fake.updateCalls).To(BeEmpty())
		})
	})
})

var _ = Describe("machineAddress", func() {
	It("returns the lease IP for the MAC", func() {
		fake := &fakeNetworkOps{
			leases: []libvirt.NetworkDHCPLease{
				{Mac: "52:54:00:aa:bb:cc", IPaddr: "192.168.122.55"},
			},
		}

		address, err := machineAddress(fake, "testnet", "52:54:00:aa:bb:cc")
		Expect(err).NotTo(HaveOccurred())
		Expect(address).To(Equal("192.168.122.55"))
	})

	It("returns empty when there is no lease yet", func() {
		fake := &fakeNetworkOps{}

		address, err := machineAddress(fake, "testnet", "52:54:00:aa:bb:cc")
		Expect(err).NotTo(HaveOccurred())
		Expect(address).To(BeEmpty())
	})

	When("the lease lookup fails", func() {
		It("returns the error", func() {
			fake := &fakeNetworkOps{leasesErr: fmt.Errorf("dhcp: not yet started")}

			_, err := machineAddress(fake, "testnet", "52:54:00:aa:bb:cc")
			Expect(err).To(HaveOccurred())
		})
	})
})

func parseIPNum(ip string) uint32 {
	parsed := net.ParseIP(ip).To4()
	return binaryBE32(parsed)
}

func binaryBE32(ip net.IP) uint32 {
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}
