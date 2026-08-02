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

package controller

import (
	"sync"

	"github.com/thebhdn/cluster-api-provider-libvirt/internal/libvirtclient"
)

// MockInfraProvider is a test double for infraProvider.
type MockInfraProvider struct {
	mu             sync.RWMutex
	EnsureInfraErr error
}

var _ infraProvider = (*MockInfraProvider)(nil)

// SetEnsureInfraErr configures the error returned by EnsureInfra.
func (m *MockInfraProvider) SetEnsureInfraErr(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.EnsureInfraErr = err
}

// EnsureInfra returns a pre-configured error, or nil.
func (m *MockInfraProvider) EnsureInfra(cfg libvirtclient.InfraConfig) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.EnsureInfraErr
}

// MockMachineProvider is a test double for machineProvider.
type MockMachineProvider struct {
	mu                 sync.RWMutex
	GetMachineStateErr error
	CreateMachineErr   error
	StartMachineErr    error
	DeleteMachineErr   error
	MachineState       libvirtclient.DomainState
}

var _ machineProvider = &MockMachineProvider{}

// SetGetMachineStateErr configures the error returned by GetMachineState.
func (m *MockMachineProvider) SetGetMachineStateErr(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GetMachineStateErr = err
}

// SetMachineState sets the domain state returned by GetMachineState.
func (m *MockMachineProvider) SetMachineState(state libvirtclient.DomainState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.MachineState = state
}

// SetCreateMachineErr configures the error returned by CreateMachine.
func (m *MockMachineProvider) SetCreateMachineErr(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CreateMachineErr = err
}

// SetStartMachineErr configures the error returned by StartMachine.
func (m *MockMachineProvider) SetStartMachineErr(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.StartMachineErr = err
}

// SetDeleteMachineErr configures the error returned by DeleteMachine.
func (m *MockMachineProvider) SetDeleteMachineErr(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.DeleteMachineErr = err
}

// GetMachineState returns a pre-configured state and error.
func (m *MockMachineProvider) GetMachineState(cfg libvirtclient.MachineConfig) (libvirtclient.DomainState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.MachineState, m.GetMachineStateErr
}

// CreateMachine returns a pre-configured error.
func (m *MockMachineProvider) CreateMachine(cfg libvirtclient.MachineConfig) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.CreateMachineErr
}

// StartMachine returns a pre-configured error.
func (m *MockMachineProvider) StartMachine(cfg libvirtclient.MachineConfig) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.StartMachineErr
}

// DeleteMachine returns a pre-configured error.
func (m *MockMachineProvider) DeleteMachine(cfg libvirtclient.MachineConfig) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.DeleteMachineErr
}
