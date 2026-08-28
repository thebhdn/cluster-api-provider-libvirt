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
	"fmt"

	"github.com/thebhdn/cluster-api-provider-libvirt/pkg/utils"
	libvirtxml "libvirt.org/go/libvirtxml"
)

const (
	DefaultNetworkForwardMode = "nat"
	DefaultNetworkFamily      = "inet"
)

type NetworkBuilder struct {
	network *libvirtxml.Network
}

func NewNetwork(name, cidr string) (*NetworkBuilder, error) {
	gateway, prefix, err := utils.FirstHostAndPrefix(cidr)
	if err != nil {
		return nil, fmt.Errorf("build network %q: %w", name, err)
	}

	return &NetworkBuilder{
		network: &libvirtxml.Network{
			Name: name,
			Forward: &libvirtxml.NetworkForward{
				Mode: DefaultNetworkForwardMode,
			},
			IPs: []libvirtxml.NetworkIP{
				{
					Address: gateway,
					Family:  DefaultNetworkFamily,
					Prefix:  uint(prefix),
				},
			},
		},
	}, nil
}

func (b *NetworkBuilder) WithBridge(name string) *NetworkBuilder {
	b.network.Bridge = &libvirtxml.NetworkBridge{Name: name}
	return b
}

func (b *NetworkBuilder) WithDHCPRange(start, end string) *NetworkBuilder {
	b.network.IPs[0].DHCP = &libvirtxml.NetworkDHCP{
		Ranges: []libvirtxml.NetworkDHCPRange{
			{
				Start: start,
				End:   end,
			},
		},
	}
	return b
}

func (b *NetworkBuilder) Build() *libvirtxml.Network {
	return b.network
}

func (b *NetworkBuilder) Marshal() (string, error) {
	return b.network.Marshal()
}
