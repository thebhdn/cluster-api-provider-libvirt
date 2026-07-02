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

	build "github.com/thebhdn/cluster-api-provider-libvirt/internal/libvirtclient/builders"
	libvirt "libvirt.org/go/libvirt"
)

type InfraConfig struct {
	URI string

	DomainName string
	BasePool   string
	DomainPool string
	Network    string
}

type MachineConfig struct {
	InfraConfig

	BaseImage  string
	MemoryMiB  uint
	VCPU       uint
	DiskSize   uint64
	DiskFormat string
}

func (s *InfraConfig) connect() (*libvirt.Connect, error) {
	conn, err := libvirt.NewConnect(s.URI)
	if err != nil {
		return nil, fmt.Errorf("connect to libvirt: %w", err)
	}

	return conn, nil
}

func closeConn(conn *libvirt.Connect) {
	if conn != nil {
		_, _ = conn.Close()
	}
}

func (s *MachineConfig) domainName() string {
	return s.DomainName
}

func (s *MachineConfig) domainDiskName() string {
	return s.DomainName + ".qcow2"
}

func (s *MachineConfig) memoryMiB() uint {
	if s.MemoryMiB == 0 {
		return build.DefaultMemoryMiB
	}
	return s.MemoryMiB
}

func (s *MachineConfig) vCPU() uint {
	if s.VCPU == 0 {
		return build.DefaultCPU
	}
	return s.VCPU
}

func (s *MachineConfig) diskSizeGiB() uint64 {
	if s.DiskSize == 0 {
		return build.DefaultVolumeCapacityGiB
	}
	return s.DiskSize
}

func (s *MachineConfig) diskFormat() string {
	if s.DiskFormat == "" {
		return build.DefaultVolumeFormat
	}
	return s.DiskFormat
}

func (s *InfraConfig) networkName() string {
	return s.Network
}

func (s *InfraConfig) basePoolName() string {
	return s.BasePool
}

func (s *InfraConfig) domainPoolName() string {
	return s.DomainPool
}
