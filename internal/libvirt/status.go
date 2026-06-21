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

import "fmt"

type VMState string

const (
	VMStateNotFound VMState = "NotFound"
	VMStateRunning  VMState = "Running"
	VMStateStopped  VMState = "Stopped"
	VMStateUnknown  VMState = "Unknown"
)

func (s *Scope) VMExists() (bool, error) {
	state, err := s.GetVMState()
	if err != nil {
		return false, err
	}

	return state != VMStateNotFound, nil
}

func (s *Scope) IsVMRunning() (bool, error) {
	state, err := s.GetVMState()
	if err != nil {
		return false, err
	}

	return state == VMStateRunning, nil
}

func (s *Scope) GetVMState() (VMState, error) {
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
