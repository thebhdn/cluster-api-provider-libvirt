package libvirtclient

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	build "github.com/thebhdn/cluster-api-provider-libvirt/internal/libvirtclient/builders"
	libvirt "libvirt.org/go/libvirt"
)

func createRootDisk(basePool, domainPool *libvirt.StoragePool, cfg MachineConfig) (string, error) {
	volumePath, err := getVolumePath(domainPool, cfg.domainDiskName())
	if err != nil {
		return "", err
	}
	if volumePath != "" {
		return volumePath, nil
	}

	baseImagePath, err := getVolumePath(basePool, cfg.BaseImage)
	if err != nil {
		return "", err
	}
	if baseImagePath == "" {
		return "", fmt.Errorf("base image %q not found", cfg.BaseImage)
	}

	volumeXML, err := build.NewVolume(cfg.domainDiskName()).
		WithCapacity(cfg.diskSizeGiB(), "G").
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

func createISODisk(conn *libvirt.Connect, domainPool *libvirt.StoragePool, cfg MachineConfig) (string, error) {
	volumePath, err := getVolumePath(domainPool, cfg.isoDiskName())
	if err != nil {
		return "", err
	}
	if volumePath != "" {
		return volumePath, nil
	}

	var buff bytes.Buffer
	err = writeCloudInitISO(&buff, cfg)
	if err != nil {
		return "", fmt.Errorf("write cloud-init iso: %w", err)
	}

	volumeXML, err := build.NewVolume(cfg.isoDiskName()).
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

	vol, err := domainPool.LookupStorageVolByName(cfg.isoDiskName())
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
	var libvirtErr *libvirt.Error
	if errors.As(err, &libvirtErr) {
		return libvirtErr.Code == libvirt.ERR_NO_STORAGE_VOL
	}
	return false
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
