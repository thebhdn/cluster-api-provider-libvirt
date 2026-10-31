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

package controller

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/thebhdn/cluster-api-provider-libvirt/api/v1alpha1"
	"github.com/thebhdn/cluster-api-provider-libvirt/internal/libvirtclient"
)

// credentialsRefNameField indexes LibvirtClusters by the name of their credentials Secret.
const credentialsRefNameField = ".spec.credentialsRef.name"

// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// CredentialsLoader reads libvirt credentials from spec.credentialsRef or from the controller-wide Secret.
type CredentialsLoader struct {
	Client client.Client

	// DefaultSecret is used when a LibvirtCluster has no credentialsRef; an empty name turns it off.
	DefaultSecret types.NamespacedName
}

// Load returns the credentials for a LibvirtCluster, or nil for a plain connection.
func (l CredentialsLoader) Load(ctx context.Context, libvirtCluster *infrav1.LibvirtCluster) (*libvirtclient.Credentials, error) {
	if libvirtCluster.Spec.CredentialsRef != nil {
		return l.loadRef(ctx, libvirtCluster)
	}

	if l.DefaultSecret.Name == "" {
		return nil, nil
	}

	secret := &corev1.Secret{}
	if err := l.Client.Get(ctx, l.DefaultSecret, secret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get default credentials secret %s: %w", l.DefaultSecret, err)
	}

	creds, err := credentialsFromSecret(secret.Data)
	if err != nil {
		return nil, fmt.Errorf("default credentials secret %s: %w", l.DefaultSecret, err)
	}

	return creds, nil
}

func (l CredentialsLoader) loadRef(ctx context.Context, libvirtCluster *infrav1.LibvirtCluster) (*libvirtclient.Credentials, error) {
	key := types.NamespacedName{Namespace: libvirtCluster.Namespace, Name: libvirtCluster.Spec.CredentialsRef.Name}
	secret := &corev1.Secret{}
	if err := l.Client.Get(ctx, key, secret); err != nil {
		return nil, fmt.Errorf("get credentials secret %s: %w", key, err)
	}

	creds, err := credentialsFromSecret(secret.Data)
	if err != nil {
		return nil, fmt.Errorf("credentials secret %s: %w", key, err)
	}

	if creds == nil {
		return nil, fmt.Errorf("credentials secret %s: no TLS keys (%s) or SSH keys (%s) found",
			key, strings.Join(tlsSecretKeys, ", "), strings.Join(sshSecretKeys, ", "))
	}

	return creds, nil
}

var (
	tlsSecretKeys = []string{infrav1.CredentialsCACertKey, corev1.TLSCertKey, corev1.TLSPrivateKeyKey}
	sshSecretKeys = []string{corev1.SSHAuthPrivateKey, infrav1.CredentialsKnownHostsKey}
)

// credentialsFromSecret maps Secret keys to credentials; it returns nil when all keys are empty and fails on a partial set.
func credentialsFromSecret(data map[string][]byte) (*libvirtclient.Credentials, error) {
	creds := &libvirtclient.Credentials{}

	if hasAnyKey(data, tlsSecretKeys) {
		if err := requireKeys(data, tlsSecretKeys); err != nil {
			return nil, err
		}
		creds.TLS = &libvirtclient.TLSCredentials{
			CACert:     data[infrav1.CredentialsCACertKey],
			ClientCert: data[corev1.TLSCertKey],
			ClientKey:  data[corev1.TLSPrivateKeyKey],
		}
	}

	if hasAnyKey(data, sshSecretKeys) {
		if err := requireKeys(data, sshSecretKeys); err != nil {
			return nil, err
		}
		creds.SSH = &libvirtclient.SSHCredentials{
			PrivateKey: data[corev1.SSHAuthPrivateKey],
			KnownHosts: data[infrav1.CredentialsKnownHostsKey],
		}
	}

	if creds.TLS == nil && creds.SSH == nil {
		return nil, nil
	}

	return creds, nil
}

func hasAnyKey(data map[string][]byte, keys []string) bool {
	for _, key := range keys {
		if len(data[key]) > 0 {
			return true
		}
	}

	return false
}

func requireKeys(data map[string][]byte, keys []string) error {
	var missing []string
	for _, key := range keys {
		if len(data[key]) == 0 {
			missing = append(missing, key)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing keys: %s", strings.Join(missing, ", "))
	}

	return nil
}

// credentialsRefName is the index function for credentialsRefNameField.
func credentialsRefName(obj client.Object) []string {
	libvirtCluster, ok := obj.(*infrav1.LibvirtCluster)
	if !ok || libvirtCluster.Spec.CredentialsRef == nil {
		return nil
	}

	return []string{libvirtCluster.Spec.CredentialsRef.Name}
}

// secretToLibvirtClusters enqueues the LibvirtClusters that use a Secret as credentials.
func (l CredentialsLoader) secretToLibvirtClusters(ctx context.Context, secret client.Object) []reconcile.Request {
	opts := []client.ListOption{
		client.InNamespace(secret.GetNamespace()),
		client.MatchingFields{credentialsRefNameField: secret.GetName()},
	}
	if client.ObjectKeyFromObject(secret) == l.DefaultSecret {
		opts = nil
	}

	list := &infrav1.LibvirtClusterList{}
	if err := l.Client.List(ctx, list, opts...); err != nil {
		return nil
	}

	requests := make([]reconcile.Request, 0, len(list.Items))
	for i := range list.Items {
		requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(&list.Items[i])})
	}

	return requests
}
