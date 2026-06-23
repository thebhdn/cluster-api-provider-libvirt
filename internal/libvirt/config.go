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

package libvirt

import (
	"fmt"

	build "github.com/thebhdn/cluster-api-provider-libvirt/internal/libvirt/builders"
	libvirtClient "libvirt.org/go/libvirt"
)

type InfraConfig struct {
	URI string

	BasePoolName  string
	VMStoragePool string
	NetworkName   string
}

type MachineConfig struct {
	InfraConfig

	VMName string

	BaseImageName string
	MemoryMiB     uint
	VCPU          uint
	DiskSizeGiB   uint64
	DiskFormat    string
}

func (s *InfraConfig) connect() (*libvirtClient.Connect, error) {
	conn, err := libvirtClient.NewConnect(s.URI)
	if err != nil {
		return nil, fmt.Errorf("connect to libvirt: %w", err)
	}

	return conn, nil
}

func closeConn(conn *libvirtClient.Connect) {
	if conn != nil {
		_, _ = conn.Close()
	}
}

func (s *MachineConfig) vmName() string {
	return s.VMName
}

func (s *MachineConfig) vmDiskName() string {
	return s.VMName + ".qcow2"
}

func (s *MachineConfig) memoryMiB() uint {
	if s.MemoryMiB == 0 {
		return build.DefaultMemoryMiB
	}
	return s.MemoryMiB
}

func (s *MachineConfig) vcpu() uint {
	if s.VCPU == 0 {
		return build.DefaultVCPU
	}
	return s.VCPU
}

func (s *MachineConfig) diskSizeGiB() uint64 {
	if s.DiskSizeGiB == 0 {
		return build.DefaultVolumeCapacityGiB
	}
	return s.DiskSizeGiB
}

func (s *MachineConfig) diskFormat() string {
	if s.DiskFormat == "" {
		return build.DefaultVolumeFormat
	}
	return s.DiskFormat
}

func (s *InfraConfig) networkName() string {
	return s.NetworkName
}

func (s *InfraConfig) basePoolName() string {
	return s.BasePoolName
}

func (s *InfraConfig) vmStoragePool() string {
	return s.VMStoragePool
}
