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
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	libvirt "libvirt.org/go/libvirt"
)

var _ = Describe("isLibvirtErr", func() {
	type testCase struct {
		name        string
		errorCode   libvirt.ErrorNumber
		queryCode   libvirt.ErrorNumber
		shouldMatch bool
	}

	entries := []testCase{
		{"ERR_NO_DOMAIN matches", libvirt.ERR_NO_DOMAIN, libvirt.ERR_NO_DOMAIN, true},
		{"ERR_NO_STORAGE_POOL matches", libvirt.ERR_NO_STORAGE_POOL, libvirt.ERR_NO_STORAGE_POOL, true},
		{"ERR_NO_NETWORK matches", libvirt.ERR_NO_NETWORK, libvirt.ERR_NO_NETWORK, true},
		{"ERR_NO_STORAGE_VOL matches", libvirt.ERR_NO_STORAGE_VOL, libvirt.ERR_NO_STORAGE_VOL, true},
		{"ERR_NO_DOMAIN does not match ERR_NO_STORAGE_POOL", libvirt.ERR_NO_DOMAIN, libvirt.ERR_NO_STORAGE_POOL, false},
	}

	DescribeTable(
		"matches against libvirt error codes",
		func(tc testCase) {
			err := &libvirt.Error{Code: tc.errorCode}
			Expect(isLibvirtErr(fmt.Errorf("%w", err), tc.queryCode)).To(Equal(tc.shouldMatch))
		},
		Entry("ERR_NO_DOMAIN matches", entries[0]),
		Entry("ERR_NO_STORAGE_POOL matches", entries[1]),
		Entry("ERR_NO_NETWORK matches", entries[2]),
		Entry("ERR_NO_STORAGE_VOL matches", entries[3]),
		Entry("ERR_NO_DOMAIN does not match ERR_NO_STORAGE_POOL", entries[4]),
	)

	When("the error is not a libvirt error", func() {
		It("returns false", func() {
			Expect(isLibvirtErr(fmt.Errorf("standard error"), libvirt.ERR_NO_DOMAIN)).To(BeFalse())
		})
	})

	When("the error is nil", func() {
		It("returns false", func() {
			Expect(isLibvirtErr(nil, libvirt.ERR_NO_DOMAIN)).To(BeFalse())
		})
	})
})

var _ = Describe("infra sentinel errors", func() {
	DescribeTable(
		"infraCheckError classifies resource state",
		func(kind, name string, exists, active, expectMissing, expectInactive bool) {
			err := infraCheckError(kind, name, exists, active)
			var missing *ErrInfraMissing
			var inactive *ErrInfraInactive

			if expectMissing {
				Expect(err).NotTo(BeNil())
				Expect(errors.As(err, &missing)).To(BeTrue())
				Expect(missing.Kind).To(Equal(kind))
				Expect(missing.Name).To(Equal(name))
			} else if expectInactive {
				Expect(err).NotTo(BeNil())
				Expect(errors.As(err, &inactive)).To(BeTrue())
				Expect(inactive.Kind).To(Equal(kind))
				Expect(inactive.Name).To(Equal(name))
			} else {
				Expect(err).To(BeNil())
			}
		},
		Entry("missing storage pool", InfraKindStoragePool, "base-pool", false, false, true, false),
		Entry("missing network", InfraKindNetwork, "mgmt", false, false, true, false),
		Entry("inactive storage pool", InfraKindStoragePool, "domain-pool", true, false, false, true),
		Entry("inactive network", InfraKindNetwork, "mgmt", true, false, false, true),
		Entry("active storage pool", InfraKindStoragePool, "base-pool", true, true, false, false),
		Entry("active network", InfraKindNetwork, "mgmt", true, true, false, false),
	)
})
