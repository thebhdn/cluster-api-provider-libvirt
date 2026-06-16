/*
Copyright 2026.

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
	// ProviderID is the unique identifier for the machine
	// Example: libvirt://default/foo-control-plane-0
	// +optional
	ProviderID *string `json:"providerID,omitempty"`

	// VCPU is the number of virtual CPUs
	// +optional
	VCPU int32 `json:"vcpu,omitempty"`

	// MemoryMiB is VM memory in MiB
	// +optional
	MemoryMiB int32 `json:"memoryMiB,omitempty"`

	// DiskGiB is VM disk size in GiB
	// +optional
	DiskGiB int32 `json:"diskGiB,omitempty"`

	// Image is the base image path or URL
	// +optional
	Image string `json:"image,omitempty"`
}

// LibvirtMachineStatus defines the observed state of LibvirtMachine.
type LibvirtMachineStatus struct {
	// Ready indicates the VM infrastructure is ready.
	// +optional
	Ready bool `json:"ready,omitempty"`

	// Addresses contains machine IP addresses.
	// +optional
	Addresses []clusterv1.MachineAddress `json:"addresses,omitempty"`

	// Conditions represent the latest available observations of the LibvirtMachine state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

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
