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
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/diskfs/go-diskfs"
	"github.com/diskfs/go-diskfs/disk"
	"github.com/diskfs/go-diskfs/filesystem"
	"github.com/diskfs/go-diskfs/filesystem/iso9660"
	"sigs.k8s.io/yaml"
)

const (
	isoFileType      = ".iso"
	userDataFileName = "user-data"
	metaDataFileName = "meta-data"
	isoLabel         = "cidata"
)

// cloudInitMetaData is the NoCloud meta-data document.
type cloudInitMetaData struct {
	InstanceID    string `json:"instance-id"`
	LocalHostname string `json:"local-hostname"`
}

const (
	isoDiskSize         int64 = 10 * 1024 * 1024
	isoLogicalBlockSize       = 2048
)

func writeCloudInitISO(dst io.Writer, cfg MachineConfig) error {
	tmpDir, err := os.MkdirTemp("", "iso-*")
	if err != nil {
		return fmt.Errorf("create temporary dir for ISO file: %w", err)
	}
	// Best effort, the dir only holds the scratch ISO.
	defer func() { _ = os.RemoveAll(tmpDir) }()

	isoPath := filepath.Join(tmpDir, cfg.isoDiskName())
	if err := buildCloudInitISO(isoPath, cfg); err != nil {
		return err
	}
	return copyFile(dst, isoPath)
}

// buildCloudInitISO writes a NoCloud ISO with user-data and meta-data to isoPath.
func buildCloudInitISO(isoPath string, cfg MachineConfig) (err error) {
	metadata, err := yaml.Marshal(cloudInitMetaData{
		InstanceID:    cfg.instanceID(),
		LocalHostname: cfg.hostname(),
	})
	if err != nil {
		return fmt.Errorf("marshal cloud-init meta-data: %w", err)
	}

	cloudDisk, err := diskfs.Create(isoPath, isoDiskSize, diskfs.SectorSizeDefault)
	if err != nil {
		return fmt.Errorf("create ISO disk: %w", err)
	}
	defer func() {
		if closeErr := cloudDisk.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close ISO disk: %w", closeErr))
		}
	}()

	cloudDisk.LogicalBlocksize = isoLogicalBlockSize

	fs, err := cloudDisk.CreateFilesystem(disk.FilesystemSpec{
		Partition: 0,
		FSType:    filesystem.TypeISO9660,
	})
	if err != nil {
		return fmt.Errorf("create ISO filesystem: %w", err)
	}

	if err := populateISO(fs, cfg.UserData, metadata); err != nil {
		return errors.Join(err, fs.Close())
	}
	if err := fs.Close(); err != nil {
		return fmt.Errorf("close ISO filesystem: %w", err)
	}
	return nil
}

func populateISO(fs filesystem.FileSystem, userData, metaData []byte) error {
	if err := writeISOFile(fs, userDataFileName, userData); err != nil {
		return err
	}
	if err := writeISOFile(fs, metaDataFileName, metaData); err != nil {
		return err
	}

	iso, ok := fs.(*iso9660.FileSystem)
	if !ok {
		return fmt.Errorf("not an iso9660 filesystem")
	}
	if err := iso.Finalize(iso9660.FinalizeOptions{
		VolumeIdentifier: isoLabel,
		RockRidge:        true,
	}); err != nil {
		return fmt.Errorf("finalize ISO: %w", err)
	}
	return nil
}

func writeISOFile(fs filesystem.FileSystem, name string, data []byte) error {
	f, err := fs.OpenFile(name, os.O_CREATE|os.O_WRONLY)
	if err != nil {
		return fmt.Errorf("open cloud-init %s file: %w", name, err)
	}
	if _, err := f.Write(data); err != nil {
		return errors.Join(fmt.Errorf("write cloud-init %s: %w", name, err), f.Close())
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close cloud-init %s file: %w", name, err)
	}
	return nil
}

func copyFile(dst io.Writer, path string) error {
	src, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open iso file: %w", err)
	}
	// Read only, a close error cannot lose data.
	defer func() { _ = src.Close() }()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("copy ISO data: %w", err)
	}
	return nil
}
