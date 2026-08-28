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

package utils

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("FirstHostAndPrefix", func() {
	Context("valid IPv4 CIDRs", func() {
		entries := []struct {
			cidr   string
			host   string
			prefix int
		}{
			{"192.168.112.0/24", "192.168.112.1", 24},
			{"10.10.0.0/16", "10.10.0.1", 16},
			{"172.16.5.33/24", "172.16.5.1", 24},
			{"10.0.0.255/24", "10.0.0.1", 24},
		}

		for _, entry := range entries {
			entry := entry
			It("returns the first usable host and prefix for "+entry.cidr, func() {
				host, prefix, err := FirstHostAndPrefix(entry.cidr)

				Expect(err).To(BeNil())
				Expect(host).To(Equal(entry.host))
				Expect(prefix).To(Equal(entry.prefix))
			})
		}
	})

	Context("invalid input", func() {
		It("returns an error for a non-CIDR string", func() {
			_, _, err := FirstHostAndPrefix("not-a-cidr")

			Expect(err).NotTo(BeNil())
		})

		It("returns an error for an IPv6 CIDR", func() {
			_, _, err := FirstHostAndPrefix("fd00::/64")

			Expect(err).NotTo(BeNil())
		})

		It("returns an error for a /32 with no usable hosts", func() {
			_, _, err := FirstHostAndPrefix("10.0.0.1/32")

			Expect(err).NotTo(BeNil())
		})
	})
})
