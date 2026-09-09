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
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"net"
	"strings"

	libvirt "libvirt.org/go/libvirt"
	libvirtxml "libvirt.org/go/libvirtxml"
)

// machineReservationBlockSize is how many of the highest addresses of the
// network's DHCP range (or subnet when no range is defined) are used for
// machine static reservations.
const machineReservationBlockSize = 200

// networkOperations is the narrow surface of *libvirt.Network that machine
// networking needs. It is an interface so the wrapper functions can be unit
// tested without a live libvirt connection; the pure allocation/lease logic
// operates on parsed data and is tested directly.
type networkOperations interface {
	GetXMLDesc(flags libvirt.NetworkXMLFlags) (string, error)
	Update(cmd libvirt.NetworkUpdateCommand, section libvirt.NetworkUpdateSection, parentIndex int, xml string, flags libvirt.NetworkUpdateFlags) error
	GetDHCPLeases() ([]libvirt.NetworkDHCPLease, error)
}

// deriveDomainMAC returns a deterministic MAC address for a domain name: the
// libvirt conventional vendor-local prefix 52:54:00 plus three bytes derived
// from the name. Deterministic per domain so DHCP reservations and lease
// matching survive restarts and reboots.
func deriveDomainMAC(domainName string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(domainName))
	sum := h.Sum64()

	mac := []byte{
		0x52, 0x54, 0x00, // libvirt vendor-local prefix, locally administered
		byte(sum),
		byte(sum >> 8),
		byte(sum >> 16),
	}

	return net.HardwareAddr(mac).String()
}

func hashDomainName(domainName string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(domainName))
	return h.Sum64()
}

// hostEntryInNetwork returns the DHCP host entry bound to mac, or nil if none.
func hostEntryInNetwork(network *libvirtxml.Network, mac string) *libvirtxml.NetworkDHCPHost {
	for i := range network.IPs {
		if network.IPs[i].DHCP == nil {
			continue
		}
		for j := range network.IPs[i].DHCP.Hosts {
			host := &network.IPs[i].DHCP.Hosts[j]
			if strings.EqualFold(host.MAC, mac) {
				return host
			}
		}
	}

	return nil
}

// hostIPSet maps each configured host IP to the MAC bound to it.
func hostIPSet(network *libvirtxml.Network) map[string]string {
	set := make(map[string]string)
	for i := range network.IPs {
		if network.IPs[i].DHCP == nil {
			continue
		}
		for j := range network.IPs[i].DHCP.Hosts {
			host := &network.IPs[i].DHCP.Hosts[j]
			if host.IP == "" {
				continue
			}
			set[host.IP] = host.MAC
		}
	}

	return set
}

// leaseAddressForMac returns the IP from the DHCP leases bound to mac, or ""
// when no lease exists yet.
func leaseAddressForMac(leases []libvirt.NetworkDHCPLease, mac string) string {
	for _, lease := range leases {
		if strings.EqualFold(lease.Mac, mac) && lease.IPaddr != "" {
			return lease.IPaddr
		}
	}

	return ""
}

// planHostUpdate decides what to do for the machine's static DHCP host entry:
// the IP to bind, the host XML, the update command, and whether an update is
// needed at all (idempotent no-op when the entry already binds the MAC to the
// same IP).
func planHostUpdate(network *libvirtxml.Network, domainName, mac string) (string, string, libvirt.NetworkUpdateCommand, bool, error) {
	ip, err := allocateMachineIPIn(network, domainName, mac)
	if err != nil {
		return "", "", libvirt.NETWORK_UPDATE_COMMAND_NONE, false, err
	}

	existing := hostEntryInNetwork(network, mac)
	if existing != nil && existing.IP != "" && strings.EqualFold(existing.IP, ip) {
		// Already reserved: same MAC, same IP. Nothing to do.
		return ip, "", libvirt.NETWORK_UPDATE_COMMAND_NONE, false, nil
	}

	xml, err := machineHostXML(domainName, mac, ip)
	if err != nil {
		return "", "", libvirt.NETWORK_UPDATE_COMMAND_NONE, false, fmt.Errorf("marshal DHCP host entry for %s: %w", domainName, err)
	}

	cmd := libvirt.NETWORK_UPDATE_COMMAND_ADD_LAST
	if existing != nil {
		// MAC already bound to a different IP: re-define it.
		cmd = libvirt.NETWORK_UPDATE_COMMAND_MODIFY
	}

	return ip, xml, cmd, true, nil
}

// reserveMachineIP guarantees a static DHCP host entry for the machine on the
// network and returns the bound IP. Idempotent: an existing entry for the MAC
// with the same IP results in no update.
func reserveMachineIP(n networkOperations, netName, domainName, mac string) (string, error) {
	network, err := networkXML(n)
	if err != nil {
		return "", err
	}

	ip, xml, cmd, doUpdate, err := planHostUpdate(network, domainName, mac)
	if err != nil {
		return "", err
	}

	if !doUpdate {
		return ip, nil
	}

	if err := n.Update(cmd, libvirt.NETWORK_SECTION_IP_DHCP_HOST, 0, xml, 0); err != nil {
		return "", fmt.Errorf("update DHCP host entry for %s on network %s: %w", domainName, netName, err)
	}

	return ip, nil
}

// releaseMachineIP removes the machine's static DHCP host entry. A missing
// entry is not an error: this runs from the machine delete path and must not
// block finalizer removal.
func releaseMachineIP(n networkOperations, netName, domainName, mac string) error {
	network, err := networkXML(n)
	if err != nil {
		return err
	}

	existing := hostEntryInNetwork(network, mac)
	if existing == nil {
		return nil
	}

	xml, err := machineHostXML(domainName, mac, existing.IP)
	if err != nil {
		return fmt.Errorf("marshal DHCP host entry for %s: %w", domainName, err)
	}

	if err := n.Update(libvirt.NETWORK_UPDATE_COMMAND_DELETE, libvirt.NETWORK_SECTION_IP_DHCP_HOST, 0, xml, 0); err != nil {
		return fmt.Errorf("remove DHCP host entry for %s on network %s: %w", domainName, netName, err)
	}

	return nil
}

// machineAddress returns the IP currently bound to the machine's MAC in the
// network's DHCP leases; empty string when no lease exists yet.
func machineAddress(n networkOperations, netName, mac string) (string, error) {
	leases, err := n.GetDHCPLeases()
	if err != nil {
		return "", fmt.Errorf("get DHCP leases of network %s: %w", netName, err)
	}

	return leaseAddressForMac(leases, mac), nil
}

// allocateMachineIPIn picks an unused IP in the network's machine reservation
// block: deterministic offset from the domain name, then sequential retry on
// collision with a host entry bound to another MAC.
func allocateMachineIPIn(network *libvirtxml.Network, domainName, mac string) (string, error) {
	candidates, err := allocationCandidates(network)
	if err != nil {
		return "", err
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("network has no usable addresses for machine reservation")
	}

	taken := hostIPSet(network)
	offset := int(hashDomainName(domainName) % uint64(len(candidates)))
	for i := range len(candidates) {
		cand := candidates[(offset+i)%len(candidates)]
		ip := formatIPv4(cand)
		owner, isTaken := taken[ip]
		if !isTaken || strings.EqualFold(owner, mac) {
			return ip, nil
		}
	}

	return "", fmt.Errorf("no free address in the machine reservation block of the network (name %s)", domainName)
}

// allocationCandidates returns candidate IPs in network order for machine
// reservations: the DHCP range of the first IPv4 subnet when defined, otherwise
// the highest block of the subnet.
func allocationCandidates(network *libvirtxml.Network) ([]uint32, error) {
	var ip *libvirtxml.NetworkIP
	for i := range network.IPs {
		if network.IPs[i].Family == "inet" {
			ip = &network.IPs[i]
			break
		}
	}
	if ip == nil {
		return nil, fmt.Errorf("network has no IPv4 subnet")
	}

	ipNet, err := parseSubnet(ip.Address, ip.Netmask)
	if err != nil {
		return nil, err
	}

	if ip.DHCP != nil {
		if start, end, ok := firstDHCPRange(ip.DHCP); ok {
			return rangeCandidates(start, end)
		}
	}

	return subnetTailCandidates(ipNet, machineReservationBlockSize)
}

// firstDHCPRange returns the first start/end pair of a DHCP range as IP
// numbers.
func firstDHCPRange(dhcp *libvirtxml.NetworkDHCP) (uint32, uint32, bool) {
	for _, rng := range dhcp.Ranges {
		s := net.ParseIP(rng.Start)
		e := net.ParseIP(rng.End)
		if s == nil || e == nil {
			continue
		}
		s4, e4 := s.To4(), e.To4()
		if s4 == nil || e4 == nil {
			continue
		}

		return binary.BigEndian.Uint32(s4), binary.BigEndian.Uint32(e4), true
	}

	return 0, 0, false
}

func rangeCandidates(start, end uint32) ([]uint32, error) {
	if end < start {
		return nil, fmt.Errorf("DHCP range end is below start")
	}

	block := make([]uint32, 0, end-start+1)
	for i := start; i <= end; i++ {
		block = append(block, i)
	}

	return block, nil
}

func subnetTailCandidates(ipNet *net.IPNet, size int) ([]uint32, error) {
	ip4 := ipNet.IP.To4()
	if ip4 == nil {
		return nil, fmt.Errorf("network subnet %s is not IPv4", ipNet.IP)
	}

	hostMask := ^binary.BigEndian.Uint32(ipNet.Mask)
	hostCount := uint64(hostMask + 1)
	if hostCount <= 2 {
		return nil, fmt.Errorf("subnet %s has no usable host addresses", ipNet)
	}

	usableCount := hostCount - 2 // exclude network (0) and broadcast addresses
	n := min(int64(size), int64(usableCount))

	networkNum := binary.BigEndian.Uint32(ip4)
	lastHost := networkNum + uint32(usableCount)
	first := lastHost - uint32(n-1)

	block := make([]uint32, 0, n)
	for i := first; i <= lastHost; i++ {
		block = append(block, i)
	}

	return block, nil
}

func parseSubnet(address, netmask string) (*net.IPNet, error) {
	ip := net.ParseIP(address)
	if ip == nil {
		return nil, fmt.Errorf("parse network address %q: invalid IP", address)
	}
	maskIP := net.ParseIP(netmask)
	if maskIP == nil {
		return nil, fmt.Errorf("parse netmask %q: invalid IP", netmask)
	}
	ip4, mask4 := ip.To4(), maskIP.To4()
	if ip4 == nil || mask4 == nil {
		return nil, fmt.Errorf("network address %q or netmask %q is not IPv4", address, netmask)
	}

	return &net.IPNet{
		IP:   ip4.Mask(net.IPv4Mask(mask4[0], mask4[1], mask4[2], mask4[3])),
		Mask: net.IPv4Mask(mask4[0], mask4[1], mask4[2], mask4[3]),
	}, nil
}

func machineHostXML(domainName, mac, ip string) (string, error) {
	return (&libvirtxml.NetworkDHCPHost{
		Name: domainName,
		MAC:  mac,
		IP:   ip,
	}).Marshal()
}

func formatIPv4(ipNum uint32) string {
	ip := make(net.IP, 4)
	binary.BigEndian.PutUint32(ip, ipNum)
	return ip.String()
}

// networkXML fetches and parses the network XML.
func networkXML(n networkOperations) (*libvirtxml.Network, error) {
	xmlDesc, err := n.GetXMLDesc(0)
	if err != nil {
		return nil, fmt.Errorf("get network XML: %w", err)
	}

	network := &libvirtxml.Network{}
	if err := network.Unmarshal(xmlDesc); err != nil {
		return nil, fmt.Errorf("parse network XML: %w", err)
	}

	return network, nil
}
