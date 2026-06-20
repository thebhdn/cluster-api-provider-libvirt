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
