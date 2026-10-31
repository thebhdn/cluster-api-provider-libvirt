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
	"os"

	build "github.com/thebhdn/cluster-api-provider-libvirt/internal/libvirtclient/builders"
	libvirt "libvirt.org/go/libvirt"
)

type InfraConfig struct {
	URI        string
	BasePool   string
	DomainPool string
	Network    string

	// Credentials are optional and only used for TLS and SSH URIs.
	Credentials *Credentials
}

type MachineConfig struct {
	InfraConfig

	DomainName string
	DomainUUID string
	Hostname   string
	BaseImage  string
	MemoryMiB  uint
	VCPU       uint
	DiskSize   uint64
	UserData   []byte
}

// connect opens a libvirt connection; credential files only live until the connection is open.
func connect(cfg InfraConfig) (*libvirt.Connect, error) {
	if cfg.Credentials == nil {
		return openConnection(cfg.URI)
	}

	dir, err := os.MkdirTemp("", credentialsDirPattern)
	if err != nil {
		return nil, fmt.Errorf("create credentials dir: %w", err)
	}
	defer os.RemoveAll(dir) //nolint:errcheck

	uri, err := cfg.Credentials.connectionURI(cfg.URI, dir)
	if err != nil {
		return nil, err
	}

	return openConnection(uri)
}

func openConnection(uri string) (*libvirt.Connect, error) {
	conn, err := libvirt.NewConnect(uri)
	if err != nil {
		return nil, fmt.Errorf("connect to libvirt: %w", err)
	}

	return conn, nil
}

func (c *MachineConfig) domainName() string {
	return c.DomainName
}

// instanceID is the cloud-init instance ID, the domain UUID when known.
func (c *MachineConfig) instanceID() string {
	if c.DomainUUID == "" {
		return c.DomainName
	}
	return c.DomainUUID
}

// hostname is the guest hostname, the domain name when not set.
func (c *MachineConfig) hostname() string {
	if c.Hostname == "" {
		return c.DomainName
	}
	return c.Hostname
}

func (c *MachineConfig) domainDiskName() string {
	return c.DomainName + ".qcow2"
}

func (c *MachineConfig) isoDiskName() string {
	return "cloudinit-" + c.DomainName + ".iso"
}

func (c *MachineConfig) memoryMiB() uint {
	if c.MemoryMiB == 0 {
		return build.DefaultMemoryMiB
	}
	return c.MemoryMiB
}

func (c *MachineConfig) vCPU() uint {
	if c.VCPU == 0 {
		return build.DefaultCPU
	}
	return c.VCPU
}

func (c *MachineConfig) diskSizeGiB() uint64 {
	if c.DiskSize == 0 {
		return build.DefaultVolumeCapacityGiB
	}
	return c.DiskSize
}

func (c *InfraConfig) networkName() string {
	return c.Network
}

func (c *InfraConfig) basePoolName() string {
	return c.BasePool
}

func (c *InfraConfig) domainPoolName() string {
	return c.DomainPool
}
