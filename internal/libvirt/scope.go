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

	"github.com/go-logr/logr"
	build "github.com/thebhdn/cluster-api-provider-libvirt/internal/libvirt/builders"
	libvirtClient "libvirt.org/go/libvirt"
)

type Config struct {
	URI string

	VMName string

	BasePoolName  string
	VMDiskPool    string
	NetworkName   string
	BaseImageName string

	MemoryMiB   uint
	VCPU        uint
	DiskSizeGiB uint64
	DiskFormat  string
}

type Scope struct {
	Config Config
	Logger logr.Logger
}

func (s *Scope) connect() (*libvirtClient.Connect, error) {
	conn, err := libvirtClient.NewConnect(s.Config.URI)
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

func (s *Scope) vmName() string {
	return s.Config.VMName
}

func (s *Scope) vmDiskName() string {
	return s.Config.VMName + ".qcow2"
}

func (s *Scope) memoryMiB() uint {
	if s.Config.MemoryMiB == 0 {
		return build.DefaultMemoryMiB
	}
	return s.Config.MemoryMiB
}

func (s *Scope) vcpu() uint {
	if s.Config.VCPU == 0 {
		return build.DefaultVCPU
	}
	return s.Config.VCPU
}

func (s *Scope) diskSizeGiB() uint64 {
	if s.Config.DiskSizeGiB == 0 {
		return build.DefaultVolumeCapacityGiB
	}
	return s.Config.DiskSizeGiB
}

func (s *Scope) diskFormat() string {
	if s.Config.DiskFormat == "" {
		return build.DefaultVolumeFormat
	}
	return s.Config.DiskFormat
}
