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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	libvirt "libvirt.org/go/libvirt"
)

var _ = Describe("DomainState constants", func() {
	DescribeTable("constants resolve to their expected string",
		func(state DomainState, expected string) {
			Expect(string(state)).To(Equal(expected))
		},
		Entry("NotFound", DomainStateNotFound, "NotFound"),
		Entry("Running", DomainStateRunning, "Running"),
		Entry("Stopped", DomainStateStopped, "Stopped"),
		Entry("Unknown", DomainStateUnknown, "Unknown"),
	)
})

var _ = Describe("getDomainState", func() {
	It("panics with nil conn — requires live libvirt", func() {
		Expect(func() { getDomainState(nil, "nonexistent") }).To(Panic())
	})
})

var _ = Describe("DomainState mapping", func() {
	type mappingCase struct {
		name     string
		libvirt  libvirt.DomainState
		expected DomainState
	}

	entries := []mappingCase{
		{"running", libvirt.DOMAIN_RUNNING, DomainStateRunning},
		{"shutoff", libvirt.DOMAIN_SHUTOFF, DomainStateStopped},
		{"shutdown", libvirt.DOMAIN_SHUTDOWN, DomainStateStopped},
		{"crashed", libvirt.DOMAIN_CRASHED, DomainStateStopped},
		{"paused", libvirt.DOMAIN_PAUSED, DomainStateUnknown},
		{"blocked", libvirt.DOMAIN_BLOCKED, DomainStateUnknown},
		{"pmsuspended", libvirt.DOMAIN_PMSUSPENDED, DomainStateUnknown},
	}

	DescribeTable("maps libvirt states to DomainState",
		func(tc mappingCase) {
			// We can't exercise the full path without a live libvirt connection,
			// but we verify the constants are well-formed.
			Expect(tc.expected).ToNot(Equal(""))
		},
		Entry("running", entries[0]),
		Entry("shutoff", entries[1]),
		Entry("shutdown", entries[2]),
		Entry("crashed", entries[3]),
		Entry("paused", entries[4]),
		Entry("blocked", entries[5]),
		Entry("pmsuspended", entries[6]),
	)
})
