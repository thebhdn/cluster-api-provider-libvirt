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

func (s *Scope) createCloudInitISO(userData string) (string, error) {
	path := filepath.Join(os.TempDir(), s.vmName()+isoFileType)

	const diskSize int64 = 10 * 1024 * 1024

	mydisk, err := diskfs.Create(path, diskSize, diskfs.SectorSizeDefault)
	if err != nil {
		return "", fmt.Errorf("create ISO disk: %w", err)
	}

	mydisk.LogicalBlocksize = 2048

	fs, err := mydisk.CreateFilesystem(disk.FilesystemSpec{
		Partition: 0,
		FSType:    filesystem.TypeISO9660,
	})
	if err != nil {
		return "", fmt.Errorf("create ISO filesystem: %w", err)
	}

	userFile, err := fs.OpenFile(userDataFileName, os.O_CREATE|os.O_RDWR)
	if err != nil {
		_ = fs.Close()
		return "", err
	}

	if _, err := userFile.Write([]byte(userData)); err != nil {
		_ = fs.Close()
		return "", err
	}

	metaFile, err := fs.OpenFile(metaDataFileName, os.O_CREATE|os.O_RDWR)
	if err != nil {
		_ = fs.Close()
		return "", err
	}

	metaData := fmt.Sprintf("instance-id: %s\nlocal-hostname: %s\n", s.vmName(), s.vmName())

	if _, err := metaFile.Write([]byte(metaData)); err != nil {
		_ = fs.Close()
		return "", err
	}

	iso, ok := fs.(*iso9660.FileSystem)
	if !ok {
		_ = fs.Close()
		return "", fmt.Errorf("not an iso9660 filesystem")
	}

	if err := iso.Finalize(iso9660.FinalizeOptions{
		VolumeIdentifier: isoLabel,
		RockRidge:        true,
	}); err != nil {
		_ = fs.Close()
		return "", err
	}

	if err := fs.Close(); err != nil {
		return "", err
	}

	return path, nil
}
