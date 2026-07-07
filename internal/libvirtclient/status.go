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

type DomainState string

const (
	DomainStateNotFound DomainState = "NotFound"
	DomainStateRunning  DomainState = "Running"
	DomainStateStopped  DomainState = "Stopped"
	DomainStateUnknown  DomainState = "Unknown"
)

func (c *MachineConfig) GetDomainState() (DomainState, error) {
	conn, err := c.connect()
	if err != nil {
		return DomainStateUnknown, err
	}
	defer closeConn(conn)

	dom, err := conn.LookupDomainByName(c.domainName())
	if err != nil {
		if isDomainNotFound(err) {
			return DomainStateNotFound, nil
		}

		return DomainStateUnknown, fmt.Errorf("lookup domain %q: %w", c.domainName(), err)
	}
	defer dom.Free()

	state, _, err := dom.GetState()
	if err != nil {
		return DomainStateUnknown, fmt.Errorf("get domain state %q: %w", c.domainName(), err)
	}

	switch state {
	case libvirt.DOMAIN_RUNNING:
		return DomainStateRunning, nil

	case libvirt.DOMAIN_SHUTOFF,
		libvirt.DOMAIN_SHUTDOWN,
		libvirt.DOMAIN_CRASHED:
		return DomainStateStopped, nil

	case libvirt.DOMAIN_PAUSED,
		libvirt.DOMAIN_BLOCKED,
		libvirt.DOMAIN_PMSUSPENDED:
		return DomainStateUnknown, nil

	default:
		return DomainStateUnknown, nil
	}
}

func isDomainNotFound(err error) bool {
	var libvirtErr libvirt.Error
	if errors.As(err, &libvirtErr) {
		return libvirtErr.Code == libvirt.ERR_NO_DOMAIN
	}
	return false
}

func (c *InfraConfig) BasePoolExists() (bool, error) {
	return c.storagePoolExists(c.basePoolName())
}

func (c *InfraConfig) VMStoragePoolExists() (bool, error) {
	return c.storagePoolExists(c.domainPoolName())
}

func (c *MachineConfig) NetworkExists() (bool, error) {
	conn, err := c.connect()
	if err != nil {
		return false, err
	}
	defer closeConn(conn)

	net, err := conn.LookupNetworkByName(c.networkName())
	if err != nil {
		return false, nil
	}
	defer net.Free()

	return true, nil
}

func (c *InfraConfig) storagePoolExists(name string) (bool, error) {
	conn, err := c.connect()
	if err != nil {
		return false, err
	}
	defer closeConn(conn)

	pool, err := conn.LookupStoragePoolByName(name)
	if err != nil {
		return false, nil
	}
	defer pool.Free()

	return true, nil
}

func (s *InfraConfig) IsNetworkActive() (bool, error) {
	conn, err := s.connect()
	if err != nil {
		return false, err
	}
	defer closeConn(conn)

	net, err := conn.LookupNetworkByName(s.networkName())
	if err != nil {
		return false, nil
	}
	defer net.Free()

	active, err := net.IsActive()
	if err != nil {
		return false, fmt.Errorf("check network active %q: %w", s.networkName(), err)
	}

	return active, nil
}

func (s *InfraConfig) IsStoragePoolActive(name string) (bool, error) {
	conn, err := s.connect()
	if err != nil {
		return false, err
	}
	defer closeConn(conn)

	pool, err := conn.LookupStoragePoolByName(name)
	if err != nil {
		return false, nil
	}
	defer pool.Free()

	active, err := pool.IsActive()
	if err != nil {
		return false, fmt.Errorf("check storage pool active %q: %w", name, err)
	}

	return active, nil
}
