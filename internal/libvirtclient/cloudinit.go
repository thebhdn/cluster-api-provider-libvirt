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
	"io"
	"os"
	"path/filepath"

	"github.com/diskfs/go-diskfs"
	"github.com/diskfs/go-diskfs/disk"
	"github.com/diskfs/go-diskfs/filesystem"
	"github.com/diskfs/go-diskfs/filesystem/iso9660"
)

const (
	isoFileType      = ".iso"
	userDataFileName = "user-data"
	metaDataFileName = "meta-data"
	isoLabel         = "cidata"
)

func writeCloudInitISO(dst io.Writer, cfg MachineConfig) error {
	tmpDir, err := os.MkdirTemp("", "iso-*")
	if err != nil {
		return fmt.Errorf("create temporary dir for ISO file: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	isoPath := filepath.Join(tmpDir, cfg.isoDiskName())

	const diskSize int64 = 10 * 1024 * 1024

	cloudDisk, err := diskfs.Create(isoPath, diskSize, diskfs.SectorSizeDefault)
	if err != nil {
		return fmt.Errorf("create ISO disk: %w", err)
	}
	defer cloudDisk.Close()

	cloudDisk.LogicalBlocksize = 2048

	fs, err := cloudDisk.CreateFilesystem(disk.FilesystemSpec{
		Partition: 0,
		FSType:    filesystem.TypeISO9660,
	})
	if err != nil {
		return fmt.Errorf("create ISO filesystem: %w", err)
	}

	metadata := fmt.Appendf(
		nil,
		"instance-id: %s\nlocal-hostname: %s\n",
		cfg.domainName(),
		cfg.domainName(),
	)

	flags := os.O_CREATE | os.O_WRONLY

	userFile, err := fs.OpenFile(userDataFileName, flags)
	if err != nil {
		fs.Close()
		return fmt.Errorf("open cloud-init user-data file: %w", err)
	}

	if _, err = userFile.Write(cfg.UserData); err != nil {
		userFile.Close()
		fs.Close()
		return fmt.Errorf("write cloud-init user-data: %w", err)
	}

	if err = userFile.Close(); err != nil {
		fs.Close()
		return fmt.Errorf("close cloud-init user-data file: %w", err)
	}

	metaFile, err := fs.OpenFile(metaDataFileName, flags)
	if err != nil {
		fs.Close()
		return fmt.Errorf("open cloud-init meta-data file: %w", err)
	}

	if _, err = metaFile.Write(metadata); err != nil {
		metaFile.Close()
		fs.Close()
		return fmt.Errorf("write cloud-init meta-data: %w", err)
	}

	if err = metaFile.Close(); err != nil {
		fs.Close()
		return fmt.Errorf("close cloud-init meta-data file: %w", err)
	}

	iso, ok := fs.(*iso9660.FileSystem)
	if !ok {
		fs.Close()
		return fmt.Errorf("not an iso9660 filesystem")
	}

	if err = iso.Finalize(iso9660.FinalizeOptions{
		VolumeIdentifier: isoLabel,
		RockRidge:        true,
	}); err != nil {
		fs.Close()
		return fmt.Errorf("finalize ISO: %w", err)
	}

	if err = fs.Close(); err != nil {
		return fmt.Errorf("close ISO filesystem: %w", err)
	}

	isoFile, err := os.Open(isoPath)
	if err != nil {
		return fmt.Errorf("open iso file: %w", err)
	}
	defer isoFile.Close()

	if _, err := isoFile.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek ISO file: %w", err)
	}

	if _, err := io.Copy(dst, isoFile); err != nil {
		return fmt.Errorf("copy ISO data: %w", err)
	}

	return nil
}
