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
	"bytes"
	"errors"
	"fmt"
	"io"
	"time"

	build "github.com/thebhdn/cluster-api-provider-libvirt/internal/libvirtclient/builders"
	libvirt "libvirt.org/go/libvirt"
)

type InfraConfig struct {
	URI        string
	BasePool   string
	DomainPool string
	Network    string
}

type MachineConfig struct {
	InfraConfig

	DomainName string
	BaseImage  string
	MemoryMiB  uint
	VCPU       uint
	DiskSize   uint64
	DiskFormat string
	UserData   []byte
}

func (c *MachineConfig) CreateDomain() error {
	conn, err := c.connect()
	if err != nil {
		return err
	}
	defer closeConn(conn)

	domainPool, err := conn.LookupStoragePoolByName(c.DomainPool)
	if err != nil {
		return fmt.Errorf("lookup domain disk pool %s: %w", c.DomainPool, err)
	}
	defer domainPool.Free()

	basePool, err := conn.LookupStoragePoolByName(c.BasePool)
	if err != nil {
		return fmt.Errorf("lookup base disk pool %s: %w", c.BasePool, err)
	}
	defer basePool.Free()

	diskPath, err := c.createRootDisk(basePool, domainPool)
	if err != nil {
		return err
	}

	cloudISOPath, err := c.createISODisk(conn, domainPool)
	if err != nil {
		return err
	}

	domainXML, err := build.NewDomain(c.domainName()).
		WithMemoryMiB(c.memoryMiB()).
		WithVCPU(c.vCPU()).
		WithDiskFile(diskPath).
		WithCloudInitISO(cloudISOPath).
		WithNetwork(c.Network).
		WithSerialConsole().
		Marshal()
	if err != nil {
		return fmt.Errorf("marshal domain XML: %w", err)
	}

	domain, err := conn.DomainDefineXML(domainXML)
	if err != nil {
		return fmt.Errorf("define domain %s: %w", c.domainName(), err)
	}
	defer domain.Free()

	if err := domain.Create(); err != nil {
		return fmt.Errorf("create domain %s: %w", c.domainName(), err)
	}

	return nil
}

func (c *MachineConfig) DeleteDomain() error {
	conn, err := c.connect()
	if err != nil {
		return err
	}
	defer closeConn(conn)

	if err := deleteDomain(conn, c.domainName()); err != nil {
		return err
	}

	if err := deleteVolume(conn, c.DomainPool, c.domainDiskName()); err != nil {
		return err
	}

	return nil
}

func (c *MachineConfig) createRootDisk(basePool, domainPool *libvirt.StoragePool) (string, error) {
	volumePath, err := getVolumePath(domainPool, c.domainDiskName())
	if err != nil {
		return "", err
	}
	if volumePath != "" {
		return volumePath, nil
	}

	baseImagePath, err := getVolumePath(basePool, c.BaseImage)
	if err != nil {
		return "", err
	}
	if baseImagePath == "" {
		return "", fmt.Errorf("base image %q not found", c.BaseImage)
	}

	volumeXML, err := build.NewVolume(c.domainDiskName()).
		WithCapacity(c.diskSizeGiB(), "G").
		WithBackingStore(baseImagePath, build.DefaultVolumeFormat).
		Marshal()
	if err != nil {
		return "", fmt.Errorf("marshal volume XML: %w", err)
	}

	volumePath, err = createVolume(domainPool, volumeXML)
	if err != nil {
		return "", fmt.Errorf("create root disk: %w", err)
	}

	return volumePath, nil
}

func (c *MachineConfig) createISODisk(conn *libvirt.Connect, domainPool *libvirt.StoragePool) (string, error) {
	volumePath, err := getVolumePath(domainPool, c.isoDiskName())
	if err != nil {
		return "", err
	}
	if volumePath != "" {
		return volumePath, nil
	}

	var buff bytes.Buffer
	err = c.writeCloudInitISO(&buff, c.UserData)
	if err != nil {
		return "", fmt.Errorf("write cloud-init iso: %w", err)
	}

	volumeXML, err := build.NewVolume(c.isoDiskName()).
		WithCapacity(uint64(buff.Len()), "bytes").
		WithRawType().
		Marshal()
	if err != nil {
		return "", fmt.Errorf("marshal volume XML: %w", err)
	}

	volumePath, err = createVolume(domainPool, volumeXML)
	if err != nil {
		return "", fmt.Errorf("create cloudinit disk: %w", err)
	}

	vol, err := domainPool.LookupStorageVolByName(c.isoDiskName())
	if err != nil {
		return "", err
	}
	defer vol.Free()

	if err := storageVolUpload(conn, buff, vol); err != nil {
		return "", fmt.Errorf("upload data to volume: %w", err)
	}

	return volumePath, nil
}

func storageVolUpload(conn *libvirt.Connect, buff bytes.Buffer, vol *libvirt.StorageVol) error {
	stream, err := conn.NewStream(0)
	if err != nil {
		return fmt.Errorf("create new stream: %w", err)
	}
	defer stream.Free()

	if err := vol.Upload(stream, 0, uint64(buff.Len()), 0); err != nil {
		return fmt.Errorf("start volume upload: %w", err)
	}

	buffReader := bytes.NewReader(buff.Bytes())

	if err := stream.SendAll(func(_ *libvirt.Stream, n int) ([]byte, error) {
		buf := make([]byte, n)

		nr, readErr := buffReader.Read(buf)
		if nr > 0 {
			return buf[:nr], nil
		}

		if readErr == io.EOF {
			return nil, nil
		}

		return nil, readErr
	}); err != nil {
		_ = stream.Abort()
		return fmt.Errorf("send volume data: %w", err)
	}

	if err := stream.Finish(); err != nil {
		return fmt.Errorf("finish volume upload: %w", err)
	}

	return nil
}

func createVolume(pool *libvirt.StoragePool, volumeXML string) (string, error) {
	vol, err := pool.StorageVolCreateXML(volumeXML, 0)
	if err != nil {
		return "", fmt.Errorf("create disk volume: %w", err)
	}
	defer vol.Free()

	path, err := vol.GetPath()
	if err != nil {
		return "", fmt.Errorf("get VM disk path: %w", err)
	}

	return path, nil
}

func getVolumePath(pool *libvirt.StoragePool, volumeName string) (string, error) {
	vol, err := pool.LookupStorageVolByName(volumeName)
	if err != nil {
		if isVolumeNotFound(err) {
			return "", nil
		}
		return "", err
	}
	defer vol.Free()

	path, err := vol.GetPath()
	if err != nil {
		return "", fmt.Errorf("get volume path for %s: %w", volumeName, err)
	}

	return path, nil
}

func isVolumeNotFound(err error) bool {
	var libvirtErr libvirt.Error
	if errors.As(err, &libvirtErr) {
		return libvirtErr.Code == libvirt.ERR_NO_STORAGE_VOL
	}
	return false
}

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

func deleteVolume(conn *libvirt.Connect, poolName, volumeName string) error {
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
		return fmt.Errorf("delete volume %s from pool %s: %w", volumeName, poolName, err)
	}

	return nil
}

func (c *InfraConfig) connect() (*libvirt.Connect, error) {
	conn, err := libvirt.NewConnect(c.URI)
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

func (c *MachineConfig) domainName() string {
	return c.DomainName
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
