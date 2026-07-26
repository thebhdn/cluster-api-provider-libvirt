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

	"libvirt.org/go/libvirt"
)

func ensureNetwork(conn *libvirt.Connect, name string) error {
	netExists, err := networkExists(conn, name)
	if err != nil {
		return err
	}
	if !netExists {
		return fmt.Errorf("libvirt network %s doesnt exist", name)
	}

	netActive, err := isNetworkActive(conn, name)
	if err != nil {
		return err
	}
	if !netActive {
		return fmt.Errorf("libvirt network %s is not active", name)
	}

	return nil
}

func ensureBasePool(conn *libvirt.Connect, name string) error {
	basePoolExists, err := storagePoolExists(conn, name)
	if err != nil {
		return err
	}
	if !basePoolExists {
		return fmt.Errorf("libvirt base pool %s doesnt exist", name)
	}

	basePoolActive, err := isStoragePoolActive(conn, name)
	if err != nil {
		return err
	}
	if !basePoolActive {
		return fmt.Errorf("libvirt base pool %s is not active", name)
	}

	return nil
}

func ensureDomainPool(conn *libvirt.Connect, name string) error {
	domainPoolExists, err := storagePoolExists(conn, name)
	if err != nil {
		return err
	}
	if !domainPoolExists {
		return fmt.Errorf("libvirt domain pool %s doesnt exist", name)
	}

	domainPoolActive, err := isStoragePoolActive(conn, name)
	if err != nil {
		return err
	}
	if !domainPoolActive {
		return fmt.Errorf("libvirt domain pool %s is not active", name)
	}

	return nil
}

func networkExists(conn *libvirt.Connect, name string) (bool, error) {
	net, err := conn.LookupNetworkByName(name)
	if err != nil {
		if isLibvirtErr(err, libvirt.ERR_NO_NETWORK) {
			return false, nil
		}
		return false, fmt.Errorf("lookup network %s: %w", name, err)
	}
	defer net.Free()

	return true, nil
}

func isNetworkActive(conn *libvirt.Connect, name string) (bool, error) {
	net, err := conn.LookupNetworkByName(name)
	if err != nil {
		return false, fmt.Errorf("lookup network %s: %w", name, err)
	}
	defer net.Free()

	active, err := net.IsActive()
	if err != nil {
		return false, fmt.Errorf("check network active %q: %w", name, err)
	}

	return active, nil
}

func storagePoolExists(conn *libvirt.Connect, name string) (bool, error) {
	pool, err := conn.LookupStoragePoolByName(name)
	if err != nil {
		if isLibvirtErr(err, libvirt.ERR_NO_STORAGE_POOL) {
			return false, nil
		}
		return false, fmt.Errorf("lookup storage-pool %s: %w", name, err)
	}
	defer pool.Free()

	return true, nil
}

func isStoragePoolActive(conn *libvirt.Connect, name string) (bool, error) {
	pool, err := conn.LookupStoragePoolByName(name)
	if err != nil {
		return false, fmt.Errorf("lookup storage-pool %s: %w", name, err)
	}
	defer pool.Free()

	active, err := pool.IsActive()
	if err != nil {
		return false, fmt.Errorf("check storage pool active %q: %w", name, err)
	}

	return active, nil
}

func isLibvirtErr(err error, code libvirt.ErrorNumber) bool {
	var libvirtErr *libvirt.Error
	return errors.As(err, &libvirtErr) && libvirtErr.Code == code
}
