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
	"encoding/binary"
	"fmt"
	"net"
	"strings"
)

func IsIP(address string) bool {
	ip := net.ParseIP(address)
	return ip != nil
}

func IsIPv4(address string) bool {
	ip := net.ParseIP(address)
	if ip == nil {
		return false
	}
	return ip.To4() != nil
}

func IsIPv6(address string) bool {
	ip := net.ParseIP(address)
	if ip == nil {
		return false
	}
	return ip.To4() == nil
}

func IsIPv4CIDR(cidr string) bool {
	ip, _, _ := net.ParseCIDR(cidr)
	if ip == nil {
		return false
	}
	return ip.To4() != nil
}

func IsIPv6CIDR(cidr string) bool {
	ip, _, _ := net.ParseCIDR(cidr)
	if ip == nil {
		return false
	}
	return ip.To4() == nil
}

func StripCIDR(ip string) string {
	if idx := strings.Index(ip, "/"); idx >= 0 {
		return ip[:idx]
	}
	return ip
}

func FirstHostAndPrefix(cidr string) (string, int, error) {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", 0, fmt.Errorf("parse CIDR %q: %w", cidr, err)
	}
	ip4 := ipnet.IP.To4()
	if ip4 == nil {
		return "", 0, fmt.Errorf("CIDR %q is not IPv4", cidr)
	}
	ones, _ := ipnet.Mask.Size()
	if ones > 30 {
		return "", 0, fmt.Errorf("CIDR %q has no usable host addresses", cidr)
	}
	// ParseCIDR returns the masked network address; the first usable
	// host is network address + 1.
	first := binary.BigEndian.Uint32(ip4) + 1
	if first == 0 {
		return "", 0, fmt.Errorf("CIDR %q has no usable host addresses", cidr)
	}
	return net.IPv4(byte(first>>24), byte(first>>16), byte(first>>8), byte(first)).String(), ones, nil
}
