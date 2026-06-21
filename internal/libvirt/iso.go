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

package main

import (
	"fmt"
	"os"
	"path/filepath"

	diskfs "github.com/diskfs/go-diskfs"
	"github.com/diskfs/go-diskfs/disk"
	"github.com/diskfs/go-diskfs/filesystem"
	"github.com/diskfs/go-diskfs/filesystem/iso9660"
)

const (
	isoFileType      = ".iso"
	usrDataFileName  = "user-data"
	metaDataFileName = "meta-data"
	isoLabel         = "cidata"
)

func createIso(isoName, userData, metaData string) (string, error) {
	path := filepath.Join(os.TempDir(), isoName+isoFileType)

	var diskSize int64 = 10 * 1024 * 1024 // 10 MB
	mydisk, err := diskfs.Create(path, diskSize, diskfs.SectorSizeDefault)
	if err != nil {
		return "", err
	}

	// the following line is required for an ISO, which may have logical block sizes
	// only of 2048, 4096, 8192
	mydisk.LogicalBlocksize = 2048
	fspec := disk.FilesystemSpec{Partition: 0, FSType: filesystem.TypeISO9660}
	fs, err := mydisk.CreateFilesystem(fspec)
	if err != nil {
		return "", err
	}

	defer func() {
		closeErr := fs.Close()
		if err == nil && closeErr != nil {
			err = closeErr
		}
	}()

	usr, err := fs.OpenFile(usrDataFileName, os.O_CREATE|os.O_RDWR)
	if err != nil {
		return "", err
	}
	_, err = usr.Write([]byte(userData))

	meta, err := fs.OpenFile(metaDataFileName, os.O_CREATE|os.O_RDWR)
	if err != nil {
		return "", err
	}
	_, err = meta.Write([]byte(metaData))

	iso, ok := fs.(*iso9660.FileSystem)
	if !ok {
		return "", fmt.Errorf("not an iso9660 filesystem")
	}
	err = iso.Finalize(iso9660.FinalizeOptions{
		VolumeIdentifier: isoLabel,
		RockRidge:        true,
	})
	if err != nil {
		return "", err
	}

	return path, nil
}
