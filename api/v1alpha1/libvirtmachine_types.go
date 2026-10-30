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

// LibvirtMachineSpec defines the desired state of LibvirtMachine
type LibvirtMachineSpec struct {
	// ProviderID is the unique identifier for the machine, set by the controller
	// Example: libvirt://3f1c0a52-6b3e-4c51-9a0e-8d7f4e2b1c90
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="providerID is immutable once set"
	ProviderID *string `json:"providerID,omitempty"`

	// VCPU is the number of virtual CPUs
	// +optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="vcpu is immutable"
	VCPU int32 `json:"vcpu,omitempty"`

	// MemoryMiB is Domain memory in MiB
	// +optional
	// +kubebuilder:default=1024
	// +kubebuilder:validation:Minimum=256
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="memoryMiB is immutable"
	MemoryMiB int32 `json:"memoryMiB,omitempty"`

	// DiskGiB is Domain disk size in GiB
	// +optional
	// +kubebuilder:default=20
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="diskGiB is immutable"
	DiskGiB int32 `json:"diskGiB,omitempty"`

	// Image is the name of the base image volume in the LibvirtCluster basePool
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="image is immutable"
	Image string `json:"image"`
}

// LibvirtMachineStatus defines the observed state of LibvirtMachine.
type LibvirtMachineStatus struct {
	// Ready indicates the Domain infrastructure is ready.
	// +optional
	Ready bool `json:"ready,omitempty"`

	// Addresses contains machine IP addresses.
	// +optional
	Addresses []clusterv1.MachineAddress `json:"addresses,omitempty"`

	// Conditions represent the latest available observations of the LibvirtMachine state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// +optional
	Initialization LibvirtMachineInitializationStatus `json:"initialization,omitempty,omitzero"`
}

type LibvirtMachineInitializationStatus struct {
	// +optional
	Provisioned bool `json:"provisioned,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels['cluster\\.x-k8s\\.io/cluster-name']",description="Cluster this LibvirtMachine belongs to"
// +kubebuilder:printcolumn:name="Provisioned",type="boolean",JSONPath=".status.initialization.provisioned",description="Domain was created"
// +kubebuilder:printcolumn:name="State",type="string",JSONPath=`.status.conditions[?(@.type=="DomainRunning")].reason`,description="Domain state on the libvirt host"
// +kubebuilder:printcolumn:name="Address",type="string",JSONPath=".status.addresses[0].address",description="Domain IP address"
// +kubebuilder:printcolumn:name="ProviderID",type="string",JSONPath=".spec.providerID",description="Provider ID of the machine",priority=1
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// LibvirtMachine is the Schema for the libvirtmachines API
type LibvirtMachine struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of LibvirtMachine
	// +required
	Spec LibvirtMachineSpec `json:"spec"`

	// status defines the observed state of LibvirtMachine
	// +optional
	Status LibvirtMachineStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// LibvirtMachineList contains a list of LibvirtMachine
type LibvirtMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []LibvirtMachine `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LibvirtMachine{}, &LibvirtMachineList{})
}

func (l *LibvirtMachine) GetConditions() []metav1.Condition {
	return l.Status.Conditions
}

// SetConditions sets the conditions on this object.
func (l *LibvirtMachine) SetConditions(conditions []metav1.Condition) {
	l.Status.Conditions = conditions
}
