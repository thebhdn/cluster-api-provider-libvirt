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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	libvirtxml "libvirt.org/go/libvirtxml"
)

var _ = Describe("StoragePoolBuilder", func() {
	Context("NewStoragePool", func() {
		It("creates an fs pool with name and target path", func() {
			pool := NewStoragePool("test-pool", "/var/lib/libvirt/test-pool")

			Expect(pool.pool.Type).To(Equal(DefaultStoragePoolType))
			Expect(pool.pool.Name).To(Equal("test-pool"))
			Expect(pool.pool.Target).NotTo(BeNil())
			Expect(pool.pool.Target.Path).To(Equal("/var/lib/libvirt/test-pool"))
		})

		It("leaves source unset for fs pools", func() {
			pool := NewStoragePool("no-source", "/var/lib/libvirt/no-source")

			Expect(pool.pool.Source).To(BeNil())
		})
	})

	Context("Build", func() {
		It("returns the configured StoragePool", func() {
			result := NewStoragePool("build-test", "/var/lib/libvirt/build-test").Build()

			Expect(result.Type).To(Equal(DefaultStoragePoolType))
			Expect(result.Name).To(Equal("build-test"))
			Expect(result.Target.Path).To(Equal("/var/lib/libvirt/build-test"))
		})
	})

	Context("Build return type", func() {
		It("returns *libvirtxml.StoragePool", func() {
			result := NewStoragePool("type-test", "/var/lib/libvirt/type-test").Build()

			Expect(result).To(BeAssignableToTypeOf((*libvirtxml.StoragePool)(nil)))
		})
	})

	Context("Marshal", func() {
		It("produces valid XML with pool root tag and required fields", func() {
			pool := NewStoragePool("marshal-test", "/var/lib/libvirt/marshal-test")

			xml, err := pool.Marshal()
			Expect(err).To(BeNil())
			Expect(xml).To(ContainSubstring(`<pool type="fs"`))
			Expect(xml).To(ContainSubstring(`<name>marshal-test</name>`))
			Expect(xml).To(ContainSubstring(`<path>/var/lib/libvirt/marshal-test</path>`))
		})

		It("does not emit a source element", func() {
			pool := NewStoragePool("no-source", "/var/lib/libvirt/no-source")

			xml, err := pool.Marshal()
			Expect(err).To(BeNil())
			Expect(xml).NotTo(ContainSubstring("<source"))
		})
	})
})
