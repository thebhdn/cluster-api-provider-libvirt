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

package builders

import libvirtxml "libvirt.org/go/libvirtxml"

const (
	DefaultStoragePoolType = "fs"
)

type StoragePoolBuilder struct {
	pool *libvirtxml.StoragePool
}

func NewStoragePool(name, targetPath string) *StoragePoolBuilder {
	return &StoragePoolBuilder{
		pool: &libvirtxml.StoragePool{
			Type: DefaultStoragePoolType,
			Name: name,
			Target: &libvirtxml.StoragePoolTarget{
				Path: targetPath,
			},
		},
	}
}

func (b *StoragePoolBuilder) Build() *libvirtxml.StoragePool {
	return b.pool
}

func (b *StoragePoolBuilder) Marshal() (string, error) {
	return b.pool.Marshal()
}
