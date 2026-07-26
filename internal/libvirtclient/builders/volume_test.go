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

import (
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	libvirtxml "libvirt.org/go/libvirtxml"
)

var _ = ginkgo.Describe("VolumeBuilder", func() {
	ginkgo.Context("NewVolume", func() {
		ginkgo.It("creates a volume with the specified name and default settings", func() {
			volume := NewVolume("test-disk")

			gomega.Expect(volume.volume.Name).To(gomega.Equal("test-disk"))
			gomega.Expect(volume.volume.Capacity.Value).To(gomega.Equal(DefaultVolumeCapacityGiB))
			gomega.Expect(volume.volume.Capacity.Unit).To(gomega.Equal(DefaultVolumeCapacityUnit))
			gomega.Expect(volume.volume.Target.Format.Type).To(gomega.Equal(DefaultVolumeFormat))
		})

		ginkgo.It("initializes type and backing store to zero values", func() {
			volume := NewVolume("default-disk")

			gomega.Expect(volume.volume.Type).To(gomega.BeEmpty())
			gomega.Expect(volume.volume.BackingStore).To(gomega.BeNil())
		})
	})

	ginkgo.Context("WithCapacity", func() {
		ginkgo.It("sets capacity and unit when specified", func() {
			volume := NewVolume("test-disk").WithCapacity(50, "G")

			gomega.Expect(volume.volume.Capacity.Value).To(gomega.Equal(uint64(50)))
			gomega.Expect(volume.volume.Capacity.Unit).To(gomega.Equal("G"))
		})

		ginkgo.It("accepts raw byte capacity", func() {
			volume := NewVolume("test-disk").WithCapacity(1048576, "bytes")

			gomega.Expect(volume.volume.Capacity.Value).To(gomega.Equal(uint64(1048576)))
			gomega.Expect(volume.volume.Capacity.Unit).To(gomega.Equal("bytes"))
		})
	})

	ginkgo.Context("WithRawType", func() {
		ginkgo.It("sets target format to raw", func() {
			volume := NewVolume("test-disk").WithRawType()

			gomega.Expect(volume.volume.Target.Format.Type).To(gomega.Equal(RawVolumeFormat))
		})
	})

	ginkgo.Context("WithBackingStore", func() {
		ginkgo.It("creates a backing store with path and format", func() {
			volume := NewVolume("test-disk").WithBackingStore("/path/to/base.qcow2", "qcow2")

			gomega.Expect(volume.volume.BackingStore).NotTo(gomega.BeNil())
			gomega.Expect(volume.volume.BackingStore.Path).To(gomega.Equal("/path/to/base.qcow2"))
			gomega.Expect(volume.volume.BackingStore.Format.Type).To(gomega.Equal("qcow2"))
		})
	})

	ginkgo.Context("Build", func() {
		ginkgo.It("returns a fully configured StorageVolume", func() {
			volume := NewVolume("test-disk").WithCapacity(20, "G").Build()

			gomega.Expect(volume.Name).To(gomega.Equal("test-disk"))
			gomega.Expect(volume.Capacity.Value).To(gomega.Equal(uint64(20)))
		})
	})

	ginkgo.Context("Marshal", func() {
		ginkgo.It("produces valid XML with volume root tag", func() {
			volume := NewVolume("test-disk").WithCapacity(20, "G")

			xml, err := volume.Marshal()
			gomega.Expect(err).To(gomega.BeNil())
			gomega.Expect(xml).To(gomega.ContainSubstring("<volume>"))
			gomega.Expect(xml).To(gomega.ContainSubstring("test-disk"))
		})

		ginkgo.It("includes BackingStore tag when configured", func() {
			volume := NewVolume("test-disk").
				WithCapacity(20, "G").
				WithBackingStore("/var/lib/libvirt/images/base.qcow2", "qcow2")

			xml, err := volume.Marshal()
			gomega.Expect(err).To(gomega.BeNil())
			gomega.Expect(xml).To(gomega.ContainSubstring("backingStore"))
		})

		ginkgo.It("produces XML with correct volume name attribute", func() {
			volume := NewVolume("xml-test").WithCapacity(10, "G")

			xml, err := volume.Marshal()
			gomega.Expect(err).To(gomega.BeNil())
			gomega.Expect(xml).To(gomega.ContainSubstring(`<name>xml-test</name>`))
		})
	})

	ginkgo.Context("Builder chains", func() {
		ginkgo.It("chains multiple options correctly", func() {
			volume := NewVolume("chain-test").
				WithCapacity(40, "G").
				WithRawType().
				WithBackingStore("/base.img", "raw")

			result := volume.Build()

			gomega.Expect(result.Name).To(gomega.Equal("chain-test"))
			gomega.Expect(result.Capacity.Value).To(gomega.Equal(uint64(40)))
			gomega.Expect(result.Target.Format.Type).To(gomega.Equal(RawVolumeFormat))
			gomega.Expect(result.BackingStore).NotTo(gomega.BeNil())
			gomega.Expect(result.BackingStore.Format.Type).To(gomega.Equal("raw"))
		})
	})

	ginkgo.Context("Default values", func() {
		ginkgo.It("applies default capacity, unit, and format for new volumes", func() {
			volume := NewVolume("multi-test")

			gomega.Expect(volume.volume.Capacity.Value).To(gomega.Equal(DefaultVolumeCapacityGiB))
			gomega.Expect(volume.volume.Capacity.Unit).To(gomega.Equal(DefaultVolumeCapacityUnit))
			gomega.Expect(volume.volume.Target.Format.Type).To(gomega.Equal(DefaultVolumeFormat))
		})
	})

	ginkgo.It("Build() returns the correct libvirtxml type", func() {
		volume := NewVolume("type-test")
		result := volume.Build()

		gomega.Expect(result).To(gomega.BeAssignableToTypeOf((*libvirtxml.StorageVolume)(nil)))
	})
})

var _ = ginkgo.Describe("volume marshaling", func() {
	ginkgo.It("produces valid XML for a configured volume", func() {
		volume := NewVolume("xml-test").WithCapacity(10, "G")

		xml, err := volume.Marshal()
		gomega.Expect(err).To(gomega.BeNil())
		gomega.Expect(xml).To(gomega.ContainSubstring(`<name>xml-test</name>`))
	})

	ginkgo.It("serializes a complete volume chain", func() {
		volume := NewVolume("full-volume").
			WithCapacity(40, "G").
			WithBackingStore("/base.img", "raw")

		xml, err := volume.Marshal()
		gomega.Expect(err).To(gomega.BeNil())
		gomega.Expect(xml).To(gomega.ContainSubstring("full-volume"))
		gomega.Expect(xml).To(gomega.ContainSubstring("backingStore"))
	})
})
