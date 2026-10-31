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
	"net/url"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Credentials.connectionURI", func() {
	var dir string

	tlsCredentials := &TLSCredentials{
		CACert:     []byte("ca"),
		ClientCert: []byte("cert"),
		ClientKey:  []byte("key"),
	}
	sshCredentials := &SSHCredentials{
		PrivateKey: []byte("private"),
		KnownHosts: []byte("libvirt.lab ssh-ed25519 AAAA"),
	}

	BeforeEach(func() {
		dir = GinkgoT().TempDir()
	})

	expectFile := func(name, content string) {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(Equal(content))

		info, err := os.Stat(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(info.Mode().Perm()).To(Equal(os.FileMode(credentialsFileMode)))
	}

	queryOf := func(uri string) url.Values {
		parsed, err := url.Parse(uri)
		Expect(err).NotTo(HaveOccurred())
		return parsed.Query()
	}

	It("writes the TLS files and sets pkipath", func() {
		creds := &Credentials{TLS: tlsCredentials}

		uri, err := creds.connectionURI("qemu+tls://libvirt.lab/system", dir)
		Expect(err).NotTo(HaveOccurred())
		Expect(uri).To(HavePrefix("qemu+tls://libvirt.lab/system?"))
		Expect(queryOf(uri).Get(uriParamPKIPath)).To(Equal(dir))

		expectFile(tlsCACertFile, "ca")
		expectFile(tlsClientCertFile, "cert")
		expectFile(tlsClientKeyFile, "key")
	})

	It("keeps existing URI parameters", func() {
		creds := &Credentials{TLS: tlsCredentials}

		uri, err := creds.connectionURI("qemu+tls://libvirt.lab/system?no_verify=1", dir)
		Expect(err).NotTo(HaveOccurred())
		Expect(queryOf(uri).Get("no_verify")).To(Equal("1"))
		Expect(queryOf(uri).Get(uriParamPKIPath)).To(Equal(dir))
	})

	DescribeTable("writes the SSH files and sets keyfile and known_hosts",
		func(uri string) {
			creds := &Credentials{SSH: sshCredentials}

			result, err := creds.connectionURI(uri, dir)
			Expect(err).NotTo(HaveOccurred())

			query := queryOf(result)
			Expect(query.Get(uriParamKeyFile)).To(Equal(filepath.Join(dir, sshPrivateKeyFile)))
			Expect(query.Get(uriParamKnownHosts)).To(Equal(filepath.Join(dir, sshKnownHostsFile)))
			Expect(query.Get(uriParamSSHAuth)).To(Equal(sshAuthPrivateKey))

			expectFile(sshPrivateKeyFile, "private")
			expectFile(sshKnownHostsFile, "libvirt.lab ssh-ed25519 AAAA")
		},
		Entry("libssh", "qemu+libssh://capi@libvirt.lab/system"),
		Entry("libssh2", "qemu+libssh2://capi@libvirt.lab/system"),
	)

	It("fails when a TLS URI has only SSH credentials", func() {
		creds := &Credentials{SSH: sshCredentials}

		_, err := creds.connectionURI("qemu+tls://libvirt.lab/system", dir)
		Expect(err).To(MatchError(errNoTLSCredentials))
	})

	It("fails when an SSH URI has only TLS credentials", func() {
		creds := &Credentials{TLS: tlsCredentials}

		_, err := creds.connectionURI("qemu+libssh://capi@libvirt.lab/system", dir)
		Expect(err).To(MatchError(errNoSSHCredentials))
	})

	DescribeTable("fails for transports without credential support",
		func(uri string) {
			creds := &Credentials{TLS: tlsCredentials, SSH: sshCredentials}

			_, err := creds.connectionURI(uri, dir)
			Expect(err).To(MatchError(ContainSubstring("credentials are not supported")))
		},
		Entry("tcp", "qemu+tcp://libvirt.lab/system"),
		Entry("ssh", "qemu+ssh://capi@libvirt.lab/system"),
		Entry("local", "qemu:///system"),
	)
})

var _ = Describe("connect with credentials", func() {
	It("removes the credential files after connecting", func() {
		tmp := GinkgoT().TempDir()
		GinkgoT().Setenv("TMPDIR", tmp)

		cfg := InfraConfig{
			URI:         "qemu+tls://libvirt.invalid/system",
			Credentials: &Credentials{TLS: &TLSCredentials{CACert: []byte("ca")}},
		}

		_, err := connect(cfg)
		Expect(err).To(HaveOccurred())

		entries, err := os.ReadDir(tmp)
		Expect(err).NotTo(HaveOccurred())
		Expect(entries).To(BeEmpty())
	})
})
