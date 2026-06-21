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

import libvirtxml "libvirt.org/go/libvirtxml"

type DomainBuilder struct {
	domain *libvirtxml.Domain
}

func NewDomain(name string) *DomainBuilder {
	return &DomainBuilder{
		domain: &libvirtxml.Domain{
			Type: DefaultDomainType,
			Name: name,
			Memory: &libvirtxml.DomainMemory{
				Value: DefaultMemoryMiB,
				Unit:  DefaultMemoryUnit,
			},
			VCPU: &libvirtxml.DomainVCPU{
				Value: DefaultVCPU,
			},
			OS: &libvirtxml.DomainOS{
				Type: &libvirtxml.DomainOSType{
					Type: DefaultOSType,
					Arch: DefaultOSArch,
				},
				BootDevices: []libvirtxml.DomainBootDevice{
					{Dev: DefaultBootDevHD},
				},
			},
			Devices: &libvirtxml.DomainDeviceList{},
		},
	}
}

func (b *DomainBuilder) WithMemoryMiB(memory uint) *DomainBuilder {
	b.domain.Memory.Value = memory
	b.domain.Memory.Unit = DefaultMemoryUnit
	return b
}

func (b *DomainBuilder) WithVCPU(vcpu uint) *DomainBuilder {
	b.domain.VCPU.Value = vcpu
	return b
}

func (b *DomainBuilder) WithNetwork(name string) *DomainBuilder {
	if name == "" {
		name = DefaultNetworkName
	}

	b.domain.Devices.Interfaces = append(b.domain.Devices.Interfaces, libvirtxml.DomainInterface{
		Source: &libvirtxml.DomainInterfaceSource{
			Network: &libvirtxml.DomainInterfaceSourceNetwork{
				Network: name,
			},
		},
		Model: &libvirtxml.DomainInterfaceModel{
			Type: DefaultNetworkModel,
		},
	})

	return b
}

func (b *DomainBuilder) WithDefaultNetwork() *DomainBuilder {
	return b.WithNetwork(DefaultNetworkName)
}
