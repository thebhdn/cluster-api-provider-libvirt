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
	"time"

	build "github.com/thebhdn/cluster-api-provider-libvirt/internal/libvirtclient/builders"
	libvirt "libvirt.org/go/libvirt"
)

type DomainState string

const (
	DomainStateNotFound DomainState = "NotFound"
	DomainStateRunning  DomainState = "Running"
	DomainStateStopped  DomainState = "Stopped"
	DomainStateUnknown  DomainState = "Unknown"
)

func createDomain(conn *libvirt.Connect, cfg MachineConfig) error {
	domainPool, err := conn.LookupStoragePoolByName(cfg.DomainPool)
	if err != nil {
		return fmt.Errorf("lookup domain disk pool %s: %w", cfg.DomainPool, err)
	}
	defer domainPool.Free()

	basePool, err := conn.LookupStoragePoolByName(cfg.BasePool)
	if err != nil {
		return fmt.Errorf("lookup base disk pool %s: %w", cfg.BasePool, err)
	}
	defer basePool.Free()

	diskPath, err := createRootDisk(basePool, domainPool, cfg)
	if err != nil {
		return err
	}

	cloudISOPath, err := createISODisk(conn, domainPool, cfg)
	if err != nil {
		return err
	}

	domainXML, err := build.NewDomain(cfg.domainName()).
		WithMemoryMiB(cfg.memoryMiB()).
		WithVCPU(cfg.vCPU()).
		WithDiskFile(diskPath).
		WithCloudInitISO(cloudISOPath).
		WithNetwork(cfg.Network).
		WithSerialConsole().
		Marshal()
	if err != nil {
		return fmt.Errorf("marshal domain XML: %w", err)
	}

	domain, err := conn.DomainDefineXML(domainXML)
	if err != nil {
		return fmt.Errorf("define domain %s: %w", cfg.domainName(), err)
	}
	defer domain.Free()

	if err := domain.Create(); err != nil {
		return fmt.Errorf("create domain %s: %w", cfg.domainName(), err)
	}

	return nil
}

// func deleteDomain(conn *libvirt.Connect, cfg MachineConfig) error {
// 	conn, err := connect()
// 	if err != nil {
// 		return err
// 	}
// 	defer closeConn(conn)
//
// 	if err := deleteDomain(conn, c.domainName()); err != nil {
// 		return err
// 	}
//
// 	if err := deleteVolume(conn, c.DomainPool, c.domainDiskName()); err != nil {
// 		return err
// 	}
//
// 	return nil
// }

func deleteDomain(conn *libvirt.Connect, name string) error {
	dom, err := conn.LookupDomainByName(name)
	if err != nil {
		return nil
	}
	defer dom.Free()

	active, err := dom.IsActive()
	if err == nil && active {
		if err := dom.Destroy(); err != nil {
			return fmt.Errorf("destroy domain %s: %w", name, err)
		}

		time.Sleep(2 * time.Second)
	}

	if err := dom.Undefine(); err != nil {
		return fmt.Errorf("undefine domain %s: %w", name, err)
	}

	return nil
}

func getDomainState(conn *libvirt.Connect, name string) (DomainState, error) {
	dom, err := conn.LookupDomainByName(name)
	if err != nil {
		if isLibvirtErr(err, libvirt.ERR_NO_DOMAIN) {
			return DomainStateNotFound, nil
		}

		return DomainStateUnknown, fmt.Errorf("lookup domain %q: %w", name, err)
	}
	defer dom.Free()

	state, _, err := dom.GetState()
	if err != nil {
		return DomainStateUnknown, fmt.Errorf("get domain state %q: %w", name, err)
	}

	switch state {
	case libvirt.DOMAIN_RUNNING:
		return DomainStateRunning, nil

	case libvirt.DOMAIN_SHUTOFF,
		libvirt.DOMAIN_SHUTDOWN,
		libvirt.DOMAIN_CRASHED:
		return DomainStateStopped, nil

	case libvirt.DOMAIN_PAUSED,
		libvirt.DOMAIN_BLOCKED,
		libvirt.DOMAIN_PMSUSPENDED:
		return DomainStateUnknown, nil

	default:
		return DomainStateUnknown, nil
	}
}
