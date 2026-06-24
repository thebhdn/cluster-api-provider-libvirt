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

type VMState string

const (
	VMStateNotFound VMState = "NotFound"
	VMStateRunning  VMState = "Running"
	VMStateStopped  VMState = "Stopped"
	VMStateUnknown  VMState = "Unknown"
)

func (s *MachineConfig) VMExists() (bool, error) {
	state, err := s.getVMState()
	if err != nil {
		return false, err
	}

	return state != VMStateNotFound, nil
}

func (s *MachineConfig) IsVMRunning() (bool, error) {
	state, err := s.getVMState()
	if err != nil {
		return false, err
	}

	return state == VMStateRunning, nil
}

func (s *MachineConfig) getVMState() (VMState, error) {
	conn, err := s.connect()
	if err != nil {
		return VMStateUnknown, err
	}
	defer closeConn(conn)

	dom, err := conn.LookupDomainByName(s.vmName())
	if err != nil {
		return VMStateNotFound, nil
	}
	defer dom.Free()

	active, err := dom.IsActive()
	if err != nil {
		return VMStateUnknown, fmt.Errorf("check domain active %q: %w", s.vmName(), err)
	}

	if active {
		return VMStateRunning, nil
	}

	return VMStateStopped, nil
}

func (s *InfraConfig) BasePoolExists() (bool, error) {
	return s.storagePoolExists(s.basePoolName())
}

func (s *InfraConfig) VMStoragePoolExists() (bool, error) {
	return s.storagePoolExists(s.vmStoragePool())
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
