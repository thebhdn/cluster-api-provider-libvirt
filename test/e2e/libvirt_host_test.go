//go:build e2e

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

package e2e

import (
	"fmt"
	"strings"

	"libvirt.org/go/libvirt"
	"libvirt.org/go/libvirtxml"
)

const (
	varLibvirtURI        = "LIBVIRT_URI"
	varLibvirtHostURI    = "LIBVIRT_HOST_URI"
	varLibvirtImage      = "LIBVIRT_IMAGE"
	varLibvirtNetwork    = "LIBVIRT_NETWORK"
	varLibvirtDomainPool = "LIBVIRT_DOMAIN_POOL"
	varControlPlaneVIP   = "CONTROL_PLANE_ENDPOINT_IP"
)

// requiredVariables have no usable default in the e2e config.
var requiredVariables = []string{varLibvirtURI, varLibvirtImage, varControlPlaneVIP}

// hostLeftovers lists what is still on the libvirt host for one spec namespace.
type hostLeftovers struct {
	Domains   []string
	Volumes   []string
	DHCPHosts []string
}

func (l hostLeftovers) empty() bool {
	return len(l.Domains) == 0 && len(l.Volumes) == 0 && len(l.DHCPHosts) == 0
}

// hostURI is the URI the test process uses, which can differ from the one the controller in kind uses.
func hostURI() string {
	if uri := e2eConfig.GetVariableOrEmpty(varLibvirtHostURI); uri != "" {
		return uri
	}
	return e2eConfig.MustGetVariable(varLibvirtURI)
}

// findHostLeftovers returns the domains, domain pool volumes and DHCP hosts whose name contains
// the namespace. Domain names start with "<namespace>-", see docs/how-it-works.md.
func findHostLeftovers(namespace string) (hostLeftovers, error) {
	conn, err := libvirt.NewConnect(hostURI())
	if err != nil {
		return hostLeftovers{}, fmt.Errorf("connect to %s: %w", hostURI(), err)
	}
	defer func() { _, _ = conn.Close() }()

	match := namespace + "-"
	var out hostLeftovers

	if out.Domains, err = domainNames(conn, match); err != nil {
		return out, err
	}
	if out.Volumes, err = volumeNames(conn, e2eConfig.MustGetVariable(varLibvirtDomainPool), match); err != nil {
		return out, err
	}
	if out.DHCPHosts, err = dhcpHostNames(conn, e2eConfig.MustGetVariable(varLibvirtNetwork), match); err != nil {
		return out, err
	}
	return out, nil
}

func domainNames(conn *libvirt.Connect, match string) ([]string, error) {
	domains, err := conn.ListAllDomains(0)
	if err != nil {
		return nil, fmt.Errorf("list domains: %w", err)
	}
	var names []string
	for _, d := range domains {
		name, err := d.GetName()
		_ = d.Free()
		if err != nil {
			return nil, fmt.Errorf("get domain name: %w", err)
		}
		if strings.Contains(name, match) {
			names = append(names, name)
		}
	}
	return names, nil
}

func volumeNames(conn *libvirt.Connect, poolName, match string) ([]string, error) {
	pool, err := conn.LookupStoragePoolByName(poolName)
	if err != nil {
		return nil, fmt.Errorf("lookup pool %s: %w", poolName, err)
	}
	defer func() { _ = pool.Free() }()

	if err := pool.Refresh(0); err != nil {
		return nil, fmt.Errorf("refresh pool %s: %w", poolName, err)
	}
	vols, err := pool.ListAllStorageVolumes(0)
	if err != nil {
		return nil, fmt.Errorf("list volumes in pool %s: %w", poolName, err)
	}
	var names []string
	for _, v := range vols {
		name, err := v.GetName()
		_ = v.Free()
		if err != nil {
			return nil, fmt.Errorf("get volume name: %w", err)
		}
		if strings.Contains(name, match) {
			names = append(names, name)
		}
	}
	return names, nil
}

func dhcpHostNames(conn *libvirt.Connect, networkName, match string) ([]string, error) {
	network, err := conn.LookupNetworkByName(networkName)
	if err != nil {
		return nil, fmt.Errorf("lookup network %s: %w", networkName, err)
	}
	defer func() { _ = network.Free() }()

	raw, err := network.GetXMLDesc(0)
	if err != nil {
		return nil, fmt.Errorf("get network %s XML: %w", networkName, err)
	}
	var doc libvirtxml.Network
	if err := doc.Unmarshal(raw); err != nil {
		return nil, fmt.Errorf("parse network %s XML: %w", networkName, err)
	}

	var names []string
	for _, ip := range doc.IPs {
		if ip.DHCP == nil {
			continue
		}
		for _, h := range ip.DHCP.Hosts {
			if strings.Contains(h.Name, match) {
				names = append(names, h.Name)
			}
		}
	}
	return names, nil
}
