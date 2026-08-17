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

type MockProvider struct {
	mu                 sync.RWMutex
	EnsureInfraErr     error
	GetMachineStateErr error
	CreateMachineErr   error
	StartMachineErr    error
	DeleteMachineErr   error
	MachineState       libvirtclient.DomainState
}

var _ provider = (*MockProvider)(nil)

// SetEnsureInfraErr configures the error returned by EnsureInfra.
func (m *MockProvider) SetEnsureInfraErr(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.EnsureInfraErr = err
}

// EnsureInfra returns a pre-configured error, or nil.
func (m *MockProvider) EnsureInfra(cfg libvirtclient.InfraConfig) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.EnsureInfraErr
}

// SetGetMachineStateErr configures the error returned by GetMachineState.
func (m *MockProvider) SetGetMachineStateErr(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GetMachineStateErr = err
}

// SetMachineState sets the domain state returned by GetMachineState.
func (m *MockProvider) SetMachineState(state libvirtclient.DomainState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.MachineState = state
}

// SetCreateMachineErr configures the error returned by CreateMachine.
func (m *MockProvider) SetCreateMachineErr(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CreateMachineErr = err
}

// SetStartMachineErr configures the error returned by StartMachine.
func (m *MockProvider) SetStartMachineErr(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.StartMachineErr = err
}

// SetDeleteMachineErr configures the error returned by DeleteMachine.
func (m *MockProvider) SetDeleteMachineErr(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.DeleteMachineErr = err
}

// GetMachineState returns a pre-configured state and error.
func (m *MockProvider) GetMachineState(cfg libvirtclient.MachineConfig) (libvirtclient.DomainState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.MachineState, m.GetMachineStateErr
}

// CreateMachine returns a pre-configured error.
func (m *MockProvider) CreateMachine(cfg libvirtclient.MachineConfig) (libvirtclient.DomainInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return libvirtclient.DomainInfo{}, m.CreateMachineErr
}

// StartMachine returns a pre-configured error.
func (m *MockProvider) StartMachine(cfg libvirtclient.MachineConfig) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.StartMachineErr
}

// DeleteMachine returns a pre-configured error.
func (m *MockProvider) DeleteMachine(cfg libvirtclient.MachineConfig) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.DeleteMachineErr
}
