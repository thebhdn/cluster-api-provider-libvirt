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
