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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

// LibvirtClusterSpec defines the desired state of LibvirtCluster
type LibvirtClusterSpec struct {
	// URI is the libvirt connection URI
	// Example: qemu+tcp://libvirt.io/system
	// +required
	// +kubebuilder:validation:MinLength=1
	URI string `json:"uri"`

	// ControlPlaneEndpoint is the endpoint used to reach the workload cluster API server
	// +optional
	ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint,omitempty,omitzero"`

	// BasePool is the storage pool that holds the base images
	// +required
	// +kubebuilder:validation:MinLength=1
	BasePool string `json:"basePool"`

	// DomainPool is the storage pool for VM disks and cloud-init ISOs
	// +required
	// +kubebuilder:validation:MinLength=1
	DomainPool string `json:"domainPool"`

	// Network is the libvirt network the VMs are attached to
	// +required
	// +kubebuilder:validation:MinLength=1
	Network string `json:"network"`

	// CredentialsRef points to a Secret in the same namespace with the client credentials for a
	// qemu+tls, qemu+libssh or qemu+libssh2 URI. It overrides the controller-wide credentials Secret.
	// +optional
	CredentialsRef *CredentialsReference `json:"credentialsRef,omitempty"`
}

// CredentialsReference points to a Secret in the same namespace.
// For TLS the Secret holds ca.crt, tls.crt and tls.key; for SSH it holds ssh-privatekey and known_hosts.
type CredentialsReference struct {
	// Name of the Secret
	// +required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// Secret keys read from CredentialsRef in addition to the corev1 TLS and SSH keys.
const (
	// CredentialsCACertKey is the CA certificate that signed the libvirt server certificate.
	CredentialsCACertKey = "ca.crt"

	// CredentialsKnownHostsKey is an OpenSSH known_hosts file with the libvirt host key.
	CredentialsKnownHostsKey = "known_hosts"
)

// LibvirtClusterStatus defines the observed state of LibvirtCluster
type LibvirtClusterStatus struct {
	// Ready indicates the cluster-level infrastructure is ready
	// +optional
	Ready bool `json:"ready,omitempty"`

	// +optional
	Initialization LibvirtClusterInitializationStatus `json:"initialization,omitempty,omitzero"`

	// Conditions represent the latest available observations of the LibvirtCluster state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type LibvirtClusterInitializationStatus struct {
	// +optional
	Provisioned bool `json:"provisioned,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels['cluster\\.x-k8s\\.io/cluster-name']",description="Cluster this LibvirtCluster belongs to"
// +kubebuilder:printcolumn:name="Provisioned",type="boolean",JSONPath=".status.initialization.provisioned",description="Cluster infrastructure is ready"
// +kubebuilder:printcolumn:name="URI",type="string",JSONPath=".spec.uri",description="libvirt connection URI"
// +kubebuilder:printcolumn:name="Endpoint",type="string",JSONPath=".spec.controlPlaneEndpoint.host",description="Control plane endpoint host"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// LibvirtCluster is the Schema for the libvirtclusters API
type LibvirtCluster struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of LibvirtCluster
	// +required
	Spec LibvirtClusterSpec `json:"spec"`

	// status defines the observed state of LibvirtCluster
	// +optional
	Status LibvirtClusterStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// LibvirtClusterList contains a list of LibvirtCluster
type LibvirtClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []LibvirtCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LibvirtCluster{}, &LibvirtClusterList{})
}

func (l *LibvirtCluster) GetConditions() []metav1.Condition {
	return l.Status.Conditions
}

// SetConditions sets the conditions on this object.
func (l *LibvirtCluster) SetConditions(conditions []metav1.Condition) {
	l.Status.Conditions = conditions
}
