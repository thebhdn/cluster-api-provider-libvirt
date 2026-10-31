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
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Credentials holds the client credentials for one libvirt connection.
type Credentials struct {
	TLS *TLSCredentials
	SSH *SSHCredentials
}

// TLSCredentials are the PEM files for a qemu+tls connection.
type TLSCredentials struct {
	CACert     []byte
	ClientCert []byte
	ClientKey  []byte
}

// SSHCredentials are the private key and known hosts for a qemu+libssh or qemu+libssh2 connection.
type SSHCredentials struct {
	PrivateKey []byte
	KnownHosts []byte
}

const (
	credentialsDirPattern = "libvirt-credentials-"
	credentialsFileMode   = 0o600

	transportSeparator = "+"
	transportTLS       = "tls"
	transportLibSSH    = "libssh"
	transportLibSSH2   = "libssh2"

	// File names libvirt expects in the pkipath directory.
	tlsCACertFile     = "cacert.pem"
	tlsClientCertFile = "clientcert.pem"
	tlsClientKeyFile  = "clientkey.pem"

	sshPrivateKeyFile = "id_key"
	sshKnownHostsFile = "known_hosts"

	uriParamPKIPath    = "pkipath"
	uriParamKeyFile    = "keyfile"
	uriParamKnownHosts = "known_hosts"
	uriParamSSHAuth    = "sshauth"
	sshAuthPrivateKey  = "privkey"
)

var (
	errNoTLSCredentials = errors.New("the URI uses TLS but the credentials have no TLS keys")
	errNoSSHCredentials = errors.New("the URI uses SSH but the credentials have no SSH keys")
)

type credentialFile struct {
	name string
	data []byte
}

// connectionURI writes the credentials into dir and returns uri with the libvirt parameters that point to them.
func (c *Credentials) connectionURI(uri, dir string) (string, error) {
	parsed, err := url.Parse(uri)
	if err != nil {
		return "", fmt.Errorf("parse libvirt URI: %w", err)
	}

	params := parsed.Query()

	switch transport(parsed.Scheme) {
	case transportTLS:
		if c.TLS == nil {
			return "", errNoTLSCredentials
		}
		if err := writeCredentialFiles(dir, c.TLS.files()); err != nil {
			return "", err
		}
		params.Set(uriParamPKIPath, dir)
	case transportLibSSH, transportLibSSH2:
		if c.SSH == nil {
			return "", errNoSSHCredentials
		}
		if err := writeCredentialFiles(dir, c.SSH.files()); err != nil {
			return "", err
		}
		params.Set(uriParamKeyFile, filepath.Join(dir, sshPrivateKeyFile))
		params.Set(uriParamKnownHosts, filepath.Join(dir, sshKnownHostsFile))
		params.Set(uriParamSSHAuth, sshAuthPrivateKey)
	default:
		return "", fmt.Errorf("credentials are not supported for URI scheme %q, use qemu+tls, qemu+libssh or qemu+libssh2", parsed.Scheme)
	}

	parsed.RawQuery = params.Encode()

	return parsed.String(), nil
}

func (t *TLSCredentials) files() []credentialFile {
	return []credentialFile{
		{name: tlsCACertFile, data: t.CACert},
		{name: tlsClientCertFile, data: t.ClientCert},
		{name: tlsClientKeyFile, data: t.ClientKey},
	}
}

func (s *SSHCredentials) files() []credentialFile {
	return []credentialFile{
		{name: sshPrivateKeyFile, data: s.PrivateKey},
		{name: sshKnownHostsFile, data: s.KnownHosts},
	}
}

func writeCredentialFiles(dir string, files []credentialFile) error {
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f.name), f.data, credentialsFileMode); err != nil {
			return fmt.Errorf("write credential file %s: %w", f.name, err)
		}
	}

	return nil
}

// transport returns the part after "+" in a libvirt URI scheme, for example "tls" in "qemu+tls".
func transport(scheme string) string {
	_, after, found := strings.Cut(scheme, transportSeparator)
	if !found {
		return ""
	}

	return after
}
