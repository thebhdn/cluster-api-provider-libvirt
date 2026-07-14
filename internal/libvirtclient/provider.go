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
	"fmt"
)

type Provider struct{}

func NewProvider() *Provider {
	return &Provider{}
}

func (p *Provider) EnsureInfra(cfg InfraConfig) error {
	conn, err := connect(cfg.getURI())
	if err != nil {
		return fmt.Errorf("connect to libvirt host: %w", err)
	}
	defer conn.Close()

	if err := ensureNetwork(conn, cfg.networkName()); err != nil {
		return err
	}

	if err := ensureBasePool(conn, cfg.basePoolName()); err != nil {
		return err
	}

	if err := ensureDomainPool(conn, cfg.domainPoolName()); err != nil {
		return err
	}

	return nil
}

func (p *Provider) CreateMachine(cfg MachineConfig) error {
	conn, err := connect(cfg.getURI())
	if err != nil {
		return fmt.Errorf("connect to libvirt host: %w", err)
	}
	defer conn.Close()
	return createDomain(conn, cfg)
}

func (p *Provider) StartMachine(cfg MachineConfig) error {
	return nil
}

func (p *Provider) GetMachineState(cfg MachineConfig) (DomainState, error) {
	var state DomainState

	conn, err := connect(cfg.getURI())
	if err != nil {
		return state, fmt.Errorf("connect to libvirt host: %w", err)
	}
	defer conn.Close()

	state, err = getDomainState(conn, cfg.domainName())
	if err != nil {
		return state, err
	}

	return state, nil
}

func (p *Provider) DeleteMachine(cfg MachineConfig) error {
	conn, err := connect(cfg.getURI())
	if err != nil {
		return fmt.Errorf("connect to libvirt host: %w", err)
	}
	defer conn.Close()

	if err := deleteDomain(conn, cfg.domainName()); err != nil {
		return err
	}

	if err := deleteVolume(conn, cfg.domainPoolName(), cfg.domainDiskName()); err != nil {
		return err
	}

	return nil
}
