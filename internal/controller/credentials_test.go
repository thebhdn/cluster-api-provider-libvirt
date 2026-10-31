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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "github.com/thebhdn/cluster-api-provider-libvirt/api/v1alpha1"
	"github.com/thebhdn/cluster-api-provider-libvirt/internal/libvirtclient"
)

var _ = Describe("credentialsFromSecret", func() {
	tlsData := map[string][]byte{
		infrav1.CredentialsCACertKey: []byte("ca"),
		corev1.TLSCertKey:            []byte("cert"),
		corev1.TLSPrivateKeyKey:      []byte("key"),
	}
	sshData := map[string][]byte{
		corev1.SSHAuthPrivateKey:         []byte("private"),
		infrav1.CredentialsKnownHostsKey: []byte("hosts"),
	}

	It("reads TLS keys", func() {
		creds, err := credentialsFromSecret(tlsData)
		Expect(err).NotTo(HaveOccurred())
		Expect(creds.SSH).To(BeNil())
		Expect(creds.TLS).To(Equal(&libvirtclient.TLSCredentials{
			CACert:     []byte("ca"),
			ClientCert: []byte("cert"),
			ClientKey:  []byte("key"),
		}))
	})

	It("reads SSH keys", func() {
		creds, err := credentialsFromSecret(sshData)
		Expect(err).NotTo(HaveOccurred())
		Expect(creds.TLS).To(BeNil())
		Expect(creds.SSH).To(Equal(&libvirtclient.SSHCredentials{
			PrivateKey: []byte("private"),
			KnownHosts: []byte("hosts"),
		}))
	})

	It("reads both sets", func() {
		data := map[string][]byte{}
		for k, v := range tlsData {
			data[k] = v
		}
		for k, v := range sshData {
			data[k] = v
		}

		creds, err := credentialsFromSecret(data)
		Expect(err).NotTo(HaveOccurred())
		Expect(creds.TLS).NotTo(BeNil())
		Expect(creds.SSH).NotTo(BeNil())
	})

	It("returns nil when all keys are empty", func() {
		creds, err := credentialsFromSecret(map[string][]byte{
			infrav1.CredentialsCACertKey: {},
			corev1.SSHAuthPrivateKey:     {},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(creds).To(BeNil())
	})

	DescribeTable("rejects incomplete secrets",
		func(data map[string][]byte, message string) {
			_, err := credentialsFromSecret(data)
			Expect(err).To(MatchError(ContainSubstring(message)))
		},
		Entry("TLS without CA", map[string][]byte{
			corev1.TLSCertKey:       []byte("cert"),
			corev1.TLSPrivateKeyKey: []byte("key"),
		}, "missing keys: "+infrav1.CredentialsCACertKey),
		Entry("SSH without known_hosts", map[string][]byte{
			corev1.SSHAuthPrivateKey: []byte("private"),
		}, "missing keys: "+infrav1.CredentialsKnownHostsKey),
	)
})

var _ = Describe("LibvirtCluster credentials", func() {
	createSecret := func(data map[string][]byte) *corev1.Secret {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{GenerateName: "libvirt-credentials-", Namespace: validationNamespace},
			Data:       data,
		}
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, secret))).To(Succeed())
		})
		return secret
	}

	newCluster := func(secretName string) *infrav1.LibvirtCluster {
		cluster := &infrav1.LibvirtCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "credentials",
				Namespace:  validationNamespace,
				Finalizers: []string{infrav1.LibvirtClusterFinalizer},
			},
			Spec: newTestLibvirtClusterSpec(),
		}
		cluster.Spec.URI = "qemu+tls://libvirt.lab/system"
		cluster.Spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{Host: "192.168.122.10", Port: 6443}
		cluster.Spec.CredentialsRef = &infrav1.CredentialsReference{Name: secretName}
		return cluster
	}

	reconcile := func(cluster *infrav1.LibvirtCluster, mock *MockProvider) {
		reconciler := &LibvirtClusterReconciler{
			Client:      k8sClient,
			Scheme:      k8sClient.Scheme(),
			Provider:    mock,
			Credentials: CredentialsLoader{Client: k8sClient},
		}
		scope := &ClusterScope{
			Cluster:        &clusterv1.Cluster{},
			LibvirtCluster: cluster,
			InfraConfig:    newInfraConfig(cluster),
			Ctx:            ctx,
		}

		_, err := reconciler.reconcileNormal(scope)
		Expect(err).NotTo(HaveOccurred())
	}

	infraCondition := func(cluster *infrav1.LibvirtCluster) metav1.Condition {
		for _, c := range cluster.Status.Conditions {
			if c.Type == infrav1.InfrastructureReadyCondition {
				return c
			}
		}
		Fail("InfrastructureReady condition not set")
		return metav1.Condition{}
	}

	It("passes the Secret credentials to the provider", func() {
		secret := createSecret(map[string][]byte{
			infrav1.CredentialsCACertKey: []byte("ca"),
			corev1.TLSCertKey:            []byte("cert"),
			corev1.TLSPrivateKeyKey:      []byte("key"),
		})
		cluster := newCluster(secret.Name)
		mock := &MockProvider{}

		reconcile(cluster, mock)

		Expect(cluster.Status.Initialization.Provisioned).To(BeTrue())
		ensured := mock.GetEnsuredInfraConfig()
		Expect(ensured).NotTo(BeNil())
		Expect(ensured.Credentials.TLS.ClientCert).To(Equal([]byte("cert")))
	})

	It("uses the Secret of each cluster", func() {
		first := createSecret(map[string][]byte{
			corev1.SSHAuthPrivateKey:         []byte("first"),
			infrav1.CredentialsKnownHostsKey: []byte("hosts"),
		})
		second := createSecret(map[string][]byte{
			corev1.SSHAuthPrivateKey:         []byte("second"),
			infrav1.CredentialsKnownHostsKey: []byte("hosts"),
		})

		loader := CredentialsLoader{Client: k8sClient}
		firstCreds, err := loader.Load(ctx, newCluster(first.Name))
		Expect(err).NotTo(HaveOccurred())
		secondCreds, err := loader.Load(ctx, newCluster(second.Name))
		Expect(err).NotTo(HaveOccurred())

		Expect(firstCreds.SSH.PrivateKey).To(Equal([]byte("first")))
		Expect(secondCreds.SSH.PrivateKey).To(Equal([]byte("second")))
	})

	It("returns no credentials without credentialsRef and default Secret", func() {
		cluster := newCluster("unused")
		cluster.Spec.CredentialsRef = nil

		creds, err := CredentialsLoader{}.Load(ctx, cluster)
		Expect(err).NotTo(HaveOccurred())
		Expect(creds).To(BeNil())
	})

	It("fails when the credentialsRef Secret has no keys", func() {
		secret := createSecret(map[string][]byte{})

		_, err := CredentialsLoader{Client: k8sClient}.Load(ctx, newCluster(secret.Name))
		Expect(err).To(MatchError(ContainSubstring("no TLS keys")))
	})

	Describe("default Secret", func() {
		sshSecret := func(key string) *corev1.Secret {
			return createSecret(map[string][]byte{
				corev1.SSHAuthPrivateKey:         []byte(key),
				infrav1.CredentialsKnownHostsKey: []byte("hosts"),
			})
		}

		loaderFor := func(secret *corev1.Secret) CredentialsLoader {
			return CredentialsLoader{Client: k8sClient, DefaultSecret: client.ObjectKeyFromObject(secret)}
		}

		It("is used without credentialsRef", func() {
			loader := loaderFor(sshSecret("default"))
			cluster := newCluster("unused")
			cluster.Spec.CredentialsRef = nil

			creds, err := loader.Load(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(creds.SSH.PrivateKey).To(Equal([]byte("default")))
		})

		It("is overridden by credentialsRef", func() {
			loader := loaderFor(sshSecret("default"))
			override := sshSecret("override")

			creds, err := loader.Load(ctx, newCluster(override.Name))
			Expect(err).NotTo(HaveOccurred())
			Expect(creds.SSH.PrivateKey).To(Equal([]byte("override")))
		})

		It("means a plain connection when it has only empty keys", func() {
			loader := loaderFor(createSecret(map[string][]byte{
				infrav1.CredentialsCACertKey: {},
				corev1.TLSCertKey:            {},
				corev1.TLSPrivateKeyKey:      {},
			}))
			cluster := newCluster("unused")
			cluster.Spec.CredentialsRef = nil

			creds, err := loader.Load(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(creds).To(BeNil())
		})

		It("means a plain connection when it does not exist", func() {
			loader := CredentialsLoader{
				Client:        k8sClient,
				DefaultSecret: types.NamespacedName{Namespace: validationNamespace, Name: "does-not-exist"},
			}
			cluster := newCluster("unused")
			cluster.Spec.CredentialsRef = nil

			creds, err := loader.Load(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(creds).To(BeNil())
		})

		It("enqueues all LibvirtClusters when it changes", func() {
			secret := sshSecret("default")
			cluster := newCluster("unused")
			cluster.Spec.CredentialsRef = nil
			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())
			DeferCleanup(func() {
				cluster.Finalizers = nil
				Expect(client.IgnoreNotFound(k8sClient.Update(ctx, cluster))).To(Succeed())
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, cluster))).To(Succeed())
			})

			requests := loaderFor(secret).secretToLibvirtClusters(ctx, secret)
			Expect(requests).To(ContainElement(HaveField("NamespacedName", client.ObjectKeyFromObject(cluster))))
		})
	})

	It("is not provisioned when the Secret is missing", func() {
		cluster := newCluster("does-not-exist")
		mock := &MockProvider{}

		reconcile(cluster, mock)

		Expect(cluster.Status.Initialization.Provisioned).To(BeFalse())
		Expect(mock.GetEnsuredInfraConfig()).To(BeNil())
		cond := infraCondition(cluster)
		Expect(cond.Status).To(Equal(metav1.ConditionFalse))
		Expect(cond.Reason).To(Equal(infrav1.CredentialsUnavailableReason))
	})

	It("maps a Secret to the LibvirtClusters that use it", func() {
		cluster := newCluster("mapped-secret")
		Expect(credentialsRefName(cluster)).To(Equal([]string{"mapped-secret"}))

		cluster.Spec.CredentialsRef = nil
		Expect(credentialsRefName(cluster)).To(BeEmpty())
	})
})
