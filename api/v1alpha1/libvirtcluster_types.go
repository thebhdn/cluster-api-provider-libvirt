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
	// +optional
	URI string `json:"uri,omitempty"`

	// ControlPlaneEndpoint is the endpoint used to reach the workload cluster API server
	// +optional
	ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint,omitempty"`

	// +optional
	BasePool string `json:"basePool,omitempty"`

	// +optional
	DomainPool string `json:"domainPool,omitempty"`

	// +optional
	Network string `json:"network,omitempty"`
}

// LibvirtClusterStatus defines the observed state of LibvirtCluster
type LibvirtClusterStatus struct {
	// Ready indicates the cluster-level infrastructure is ready
	// +optional
	Ready bool `json:"ready,omitempty"`

	// Conditions represent the latest available observations of the LibvirtCluster state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

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
