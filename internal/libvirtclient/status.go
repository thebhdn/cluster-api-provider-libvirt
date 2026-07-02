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
)

type DomainState string

const (
	DomainStateNotFound DomainState = "NotFound"
	DomainStateRunning  DomainState = "Running"
	DomainStateStopped  DomainState = "Stopped"
	DomainStateUnknown  DomainState = "Unknown"
)

func (s *MachineConfig) GetDomainState() (DomainState, error) {
	conn, err := s.connect()
	if err != nil {
		return DomainStateUnknown, err
	}
	defer closeConn(conn)

	dom, err := conn.LookupDomainByName(s.domainName())
	if err != nil {
		if isDomainNotFound(err) {
			return DomainStateNotFound, nil
		}

		return DomainStateUnknown, fmt.Errorf("lookup domain %q: %w", s.domainName(), err)
	}
	defer dom.Free()

	state, _, err := dom.GetState()
	if err != nil {
		return DomainStateUnknown, fmt.Errorf("get domain state %q: %w", s.domainName(), err)
	}

	switch state {
	case libvirtClient.DOMAIN_RUNNING:
		return DomainStateRunning, nil

	case libvirtClient.DOMAIN_SHUTOFF,
		libvirtClient.DOMAIN_SHUTDOWN,
		libvirtClient.DOMAIN_CRASHED:
		return DomainStateStopped, nil

	case libvirtClient.DOMAIN_PAUSED,
		libvirtClient.DOMAIN_BLOCKED,
		libvirtClient.DOMAIN_PMSUSPENDED:
		return DomainStateUnknown, nil

	default:
		return DomainStateUnknown, nil
	}
}

func isDomainNotFound(err error) bool {
	var libvirtErr libvirtClient.Error
	if errors.As(err, &libvirtErr) {
		return libvirtErr.Code == libvirtClient.ERR_NO_DOMAIN
	}
	return false
}

func (s *InfraConfig) BasePoolExists() (bool, error) {
	return s.storagePoolExists(s.basePoolName())
}

func (s *InfraConfig) VMStoragePoolExists() (bool, error) {
	return s.storagePoolExists(s.domainPoolName())
}

func (s *MachineConfig) NetworkExists() (bool, error) {
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

	return true, nil
}

func (s *InfraConfig) storagePoolExists(name string) (bool, error) {
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
