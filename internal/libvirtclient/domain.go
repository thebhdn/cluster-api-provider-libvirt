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
	"os"
	"path/filepath"
	"time"

	build "github.com/thebhdn/cluster-api-provider-libvirt/internal/libvirtclient/builders"
	libvirtClient "libvirt.org/go/libvirt"
)

func (s *MachineConfig) CreateDomain(userData string) error {
	conn, err := s.connect()
	if err != nil {
		return err
	}
	defer closeConn(conn)

	seedPath, err := s.createCloudInitISO(userData)
	if err != nil {
		return fmt.Errorf("create cloud-init ISO: %w", err)
	}

	diskPath, err := s.createDiskFromBase(conn)
	if err != nil {
		return err
	}

	domainXML, err := build.NewDomain(s.domainName()).
		WithMemoryMiB(s.memoryMiB()).
		WithVCPU(s.vCPU()).
		WithDiskFile(diskPath).
		WithCloudInitISO(seedPath).
		WithNetwork(s.Network).
		WithSerialConsole().
		Marshal()
	if err != nil {
		return fmt.Errorf("marshal domain XML: %w", err)
	}

	dom, err := conn.DomainDefineXML(domainXML)
	if err != nil {
		return fmt.Errorf("define domain %q: %w", s.domainName(), err)
	}
	defer dom.Free()

	if err := dom.Create(); err != nil {
		return fmt.Errorf("start domain %q: %w", s.domainName(), err)
	}

	return nil
}

func (s *MachineConfig) DeleteDomain() error {
	conn, err := s.connect()
	if err != nil {
		return err
	}
	defer closeConn(conn)

	if err := s.deleteDomain(conn, s.domainName()); err != nil {
		return err
	}

	if err := s.deleteVolume(conn, s.DomainPool, s.domainDiskName()); err != nil {
		return err
	}

	_ = os.Remove(filepath.Join(os.TempDir(), s.domainName()+isoFileType))

	return nil
}

func (s *MachineConfig) createDiskFromBase(conn *libvirtClient.Connect) (string, error) {
	basePath, err := s.getVolumePath(conn, s.BasePool, s.BaseImage)
	if err != nil {
		return "", err
	}

	pool, err := conn.LookupStoragePoolByName(s.DomainPool)
	if err != nil {
		return "", fmt.Errorf("lookup VM disk pool %q: %w", s.DomainPool, err)
	}
	defer pool.Free()

	volumeXML, err := build.NewVolume(s.domainDiskName()).
		WithCapacityGiB(s.diskSizeGiB()).
		WithFormat(s.diskFormat()).
		WithBackingStore(basePath, s.diskFormat()).
		Marshal()
	if err != nil {
		return "", fmt.Errorf("marshal volume XML: %w", err)
	}

	vol, err := pool.StorageVolCreateXML(volumeXML, 0)
	if err != nil {
		return "", fmt.Errorf("create VM disk volume %q: %w", s.domainDiskName(), err)
	}
	defer vol.Free()

	path, err := vol.GetPath()
	if err != nil {
		return "", fmt.Errorf("get VM disk path: %w", err)
	}

	return path, nil
}

func (s *MachineConfig) getVolumePath(conn *libvirtClient.Connect, poolName, volumeName string) (string, error) {
	pool, err := conn.LookupStoragePoolByName(poolName)
	if err != nil {
		return "", fmt.Errorf("lookup pool %q: %w", poolName, err)
	}
	defer pool.Free()

	vol, err := pool.LookupStorageVolByName(volumeName)
	if err != nil {
		return "", fmt.Errorf("lookup volume %q in pool %q: %w", volumeName, poolName, err)
	}
	defer vol.Free()

	path, err := vol.GetPath()
	if err != nil {
		return "", fmt.Errorf("get volume path for %q: %w", volumeName, err)
	}

	return path, nil
}

func (s *MachineConfig) deleteDomain(conn *libvirtClient.Connect, name string) error {
	dom, err := conn.LookupDomainByName(name)
	if err != nil {
		return nil
	}
	defer dom.Free()

	active, err := dom.IsActive()
	if err == nil && active {
		if err := dom.Destroy(); err != nil {
			return fmt.Errorf("destroy domain %q: %w", name, err)
		}

		time.Sleep(2 * time.Second)
	}

	if err := dom.Undefine(); err != nil {
		return fmt.Errorf("undefine domain %q: %w", name, err)
	}

	return nil
}

func (s *MachineConfig) deleteVolume(conn *libvirtClient.Connect, poolName, volumeName string) error {
	pool, err := conn.LookupStoragePoolByName(poolName)
	if err != nil {
		return nil
	}
	defer pool.Free()

	vol, err := pool.LookupStorageVolByName(volumeName)
	if err != nil {
		return nil
	}
	defer vol.Free()

	if err := vol.Delete(0); err != nil {
		return fmt.Errorf("delete volume %q from pool %q: %w", volumeName, poolName, err)
	}

	return nil
}
