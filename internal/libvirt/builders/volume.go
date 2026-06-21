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

type VolumeBuilder struct {
	volume *libvirtxml.StorageVolume
}

func NewVolume(name string) *VolumeBuilder {
	return &VolumeBuilder{
		volume: &libvirtxml.StorageVolume{
			Name: name,
			Capacity: &libvirtxml.StorageVolumeSize{
				Value: DefaultVolumeCapacityGiB,
				Unit:  DefaultVolumeCapacityUnit,
			},
			Target: &libvirtxml.StorageVolumeTarget{
				Format: &libvirtxml.StorageVolumeTargetFormat{
					Type: DefaultVolumeFormat,
				},
			},
		},
	}
}

func (b *VolumeBuilder) WithCapacityGiB(size uint64) *VolumeBuilder {
	b.volume.Capacity.Value = size
	b.volume.Capacity.Unit = DefaultVolumeCapacityUnit
	return b
}

func (b *VolumeBuilder) WithCapacity(size uint64, unit string) *VolumeBuilder {
	b.volume.Capacity.Value = size
	b.volume.Capacity.Unit = unit
	return b
}

func (b *VolumeBuilder) WithFormat(format string) *VolumeBuilder {
	if format == "" {
		format = DefaultVolumeFormat
	}

	b.volume.Target.Format.Type = format
	return b
}

func (b *VolumeBuilder) WithPath(path string) *VolumeBuilder {
	b.volume.Target.Path = path
	return b
}

func (b *VolumeBuilder) Build() *libvirtxml.StorageVolume {
	return b.volume
}

func (b *VolumeBuilder) Marshal() (string, error) {
	return b.volume.Marshal()
}
