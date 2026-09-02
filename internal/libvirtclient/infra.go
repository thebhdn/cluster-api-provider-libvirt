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

const (
	InfraKindStoragePool = "storagePool"
	InfraKindNetwork     = "network"
)

// ErrInfraMissing reports that a libvirt resource referenced by the
// cluster spec does not exist. The provider never creates infra.
type ErrInfraMissing struct {
	// Kind is InfraKindStoragePool or InfraKindNetwork.
	Kind string
	// Name is the resource name as referenced in the cluster spec.
	Name string
}

func (e *ErrInfraMissing) Error() string {
	return fmt.Sprintf("%s '%s' not found", e.Kind, e.Name)
}

// ErrInfraInactive reports that a referenced libvirt resource exists
// but is not active. The provider does not manage pool/network
// lifecycle.
type ErrInfraInactive struct {
	// Kind is InfraKindStoragePool or InfraKindNetwork.
	Kind string
	// Name is the resource name as referenced in the cluster spec.
	Name string
}

func (e *ErrInfraInactive) Error() string {
	return fmt.Sprintf("%s '%s' exists but is not active (start it out-of-band; the provider does not manage pool/network lifecycle)", e.Kind, e.Name)
}

// infraCheckError classifies the state of a referenced libvirt resource.
func infraCheckError(kind, name string, exists, active bool) error {
	if !exists {
		return &ErrInfraMissing{
			Kind: kind,
			Name: name,
		}
	}
	if !active {
		return &ErrInfraInactive{
			Kind: kind,
			Name: name,
		}
	}
	return nil
}

func ensureNetwork(conn *libvirt.Connect, name string) error {
	netExists, err := networkExists(conn, name)
	if err != nil {
		return err
	}

	netActive, err := isNetworkActive(conn, name)
	if err != nil {
		return err
	}

	return infraCheckError(InfraKindNetwork, name, netExists, netActive)
}

func ensureBasePool(conn *libvirt.Connect, name string) error {
	basePoolExists, err := storagePoolExists(conn, name)
	if err != nil {
		return err
	}

	basePoolActive, err := isStoragePoolActive(conn, name)
	if err != nil {
		return err
	}

	return infraCheckError(InfraKindStoragePool, name, basePoolExists, basePoolActive)
}

func ensureDomainPool(conn *libvirt.Connect, name string) error {
	domainPoolExists, err := storagePoolExists(conn, name)
	if err != nil {
		return err
	}

	domainPoolActive, err := isStoragePoolActive(conn, name)
	if err != nil {
		return err
	}

	return infraCheckError(InfraKindStoragePool, name, domainPoolExists, domainPoolActive)
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
