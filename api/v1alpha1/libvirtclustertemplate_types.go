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

// LibvirtClusterTemplateSpec defines the desired state of LibvirtClusterTemplate
type LibvirtClusterTemplateSpec struct {
	// Template is the machine template
	Template LibvirtClusterTemplateResource `json:"template"`
}

type LibvirtClusterTemplateResource struct {
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata.
	// +optional
	ObjectMeta clusterv1.ObjectMeta `json:"metadata,omitempty"`
	// Spec is the LibvirtCluster spec.
	Spec LibvirtClusterSpec `json:"spec"`
}

// +kubebuilder:object:root=true

// LibvirtClusterTemplate is the Schema for the libvirtmachinetemplates API
type LibvirtClusterTemplate struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of LibvirtClusterTemplate
	// +required
	Spec LibvirtClusterTemplateSpec `json:"spec"`
}

// +kubebuilder:object:root=true

// LibvirtClusterTemplateList contains a list of LibvirtClusterTemplate
type LibvirtClusterTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []LibvirtClusterTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LibvirtClusterTemplate{}, &LibvirtClusterTemplateList{})
}
