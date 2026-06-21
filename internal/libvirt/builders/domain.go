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

const (
	DefaultDomainType = "kvm"
	DefaultMemoryMiB  = uint(1024)
	DefaultMemoryUnit = "MiB"
	DefaultVCPU       = uint(1)

	DefaultOSType = "hvm"
	DefaultOSArch = "x86_64"

	DefaultDiskDriverName = "qemu"
	DefaultDiskFormat     = "qcow2"
	DefaultDiskTarget     = "vda"
	DefaultDiskBus        = "virtio"

	DefaultNetworkName  = "default"
	DefaultNetworkModel = "virtio"

	DefaultCloudInitDriverName = "qemu"
	DefaultCloudInitFormat     = "raw"
	DefaultCloudInitBus        = "sata"
	DefaultCloudInitTarget     = "sda"

	DefaultBootDevHD = "hd"
)

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

func (b *DomainBuilder) WithDiskFile(path string) *DomainBuilder {
	b.domain.Devices.Disks = append(b.domain.Devices.Disks, libvirtxml.DomainDisk{
		Device: "disk",
		Driver: &libvirtxml.DomainDiskDriver{
			Name: DefaultDiskDriverName,
			Type: DefaultDiskFormat,
		},
		Source: &libvirtxml.DomainDiskSource{
			File: &libvirtxml.DomainDiskSourceFile{
				File: path,
			},
		},
		Target: &libvirtxml.DomainDiskTarget{
			Dev: DefaultDiskTarget,
			Bus: DefaultDiskBus,
		},
	})

	return b
}

func (b *DomainBuilder) WithCloudInitISO(path string) *DomainBuilder {
	b.domain.Devices.Disks = append(b.domain.Devices.Disks, libvirtxml.DomainDisk{
		Device: "cdrom",
		Driver: &libvirtxml.DomainDiskDriver{
			Name: DefaultCloudInitDriverName,
			Type: DefaultCloudInitFormat,
		},
		Source: &libvirtxml.DomainDiskSource{
			File: &libvirtxml.DomainDiskSourceFile{
				File: path,
			},
		},
		Target: &libvirtxml.DomainDiskTarget{
			Dev: DefaultCloudInitTarget,
			Bus: DefaultCloudInitBus,
		},
		ReadOnly: &libvirtxml.DomainDiskReadOnly{},
	})

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

func (b *DomainBuilder) WithSerialConsole() *DomainBuilder {
	port := uint(0)

	b.domain.Devices.Serials = append(b.domain.Devices.Serials, libvirtxml.DomainSerial{
		Target: &libvirtxml.DomainSerialTarget{
			Port: &port,
		},
	})

	b.domain.Devices.Consoles = append(b.domain.Devices.Consoles, libvirtxml.DomainConsole{
		Target: &libvirtxml.DomainConsoleTarget{
			Type: "serial",
			Port: &port,
		},
	})

	return b
}

func (b *DomainBuilder) Build() *libvirtxml.Domain {
	return b.domain
}

func (b *DomainBuilder) Marshal() (string, error) {
	return b.domain.Marshal()
}
