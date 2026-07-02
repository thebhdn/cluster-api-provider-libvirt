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

func (s *MachineConfig) createCloudInitISO(userData string) (string, error) {
	path := filepath.Join(os.TempDir(), s.domainName()+isoFileType)

	const diskSize int64 = 10 * 1024 * 1024

	cloudDisk, err := diskfs.Create(path, diskSize, diskfs.SectorSizeDefault)
	if err != nil {
		return "", fmt.Errorf("create ISO disk: %w", err)
	}

	cloudDisk.LogicalBlocksize = 2048

	fs, err := cloudDisk.CreateFilesystem(disk.FilesystemSpec{
		Partition: 0,
		FSType:    filesystem.TypeISO9660,
	})
	if err != nil {
		return "", fmt.Errorf("create ISO filesystem: %w", err)
	}

	defer func() {
		_ = fs.Close()
	}()

	if err := writeISOFile(fs, userDataFileName, []byte(userData)); err != nil {
		return "", fmt.Errorf("failed to add cloud-init user-data: %w", err)
	}

	metaData := fmt.Sprintf(
		"instance-id: %s\nlocal-hostname: %s\n",
		s.domainName(),
		s.domainName(),
	)

	if err := writeISOFile(fs, metaDataFileName, []byte(metaData)); err != nil {
		return "", fmt.Errorf("failed to add cloud-init meta-data: %w", err)
	}

	iso, ok := fs.(*iso9660.FileSystem)
	if !ok {
		return "", fmt.Errorf("not an iso9660 filesystem")
	}

	if err := iso.Finalize(iso9660.FinalizeOptions{
		VolumeIdentifier: isoLabel,
		RockRidge:        true,
	}); err != nil {
		return "", fmt.Errorf("finalize ISO: %w", err)
	}

	return path, nil
}

func writeISOFile(fs filesystem.FileSystem, name string, data []byte) error {
	file, err := fs.OpenFile(name, os.O_CREATE|os.O_RDWR)
	if err != nil {
		return err
	}

	if _, err := file.Write(data); err != nil {
		return err
	}

	return nil
}
