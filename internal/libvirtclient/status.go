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
)

type DomainState string

const (
	DomainStateNotFound DomainState = "NotFound"
	DomainStateRunning  DomainState = "Running"
	DomainStateStopped  DomainState = "Stopped"
	DomainStateUnknown  DomainState = "Unknown"
)

func (s *MachineConfig) DomainExists() (bool, error) {
	state, err := s.getDomainState()
	if err != nil {
		return false, err
	}

	return state != DomainStateNotFound, nil
}

func (s *MachineConfig) IsDomainRunning() (bool, error) {
	state, err := s.getDomainState()
	if err != nil {
		return false, err
	}

	return state == DomainStateRunning, nil
}

func (s *MachineConfig) getDomainState() (DomainState, error) {
	conn, err := s.connect()
	if err != nil {
		return DomainStateUnknown, err
	}
	defer closeConn(conn)

	dom, err := conn.LookupDomainByName(s.domainName())
	if err != nil {
		return DomainStateNotFound, nil
	}
	defer dom.Free()

	active, err := dom.IsActive()
	if err != nil {
		return DomainStateUnknown, fmt.Errorf("check domain active %q: %w", s.domainName(), err)
	}

	if active {
		return DomainStateRunning, nil
	}

	return DomainStateStopped, nil
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
