/*
Copyright The Kubernetes Authors.

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

package v1beta1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Extension is a single extension specification.
type Extension struct {
	// +required
	InstanceNameRegex string `json:"instanceNameRegex"`
	// +optional
	RequiredNodePoolLabels map[string]string `json:"requiredNodePoolLabels,omitempty"`
	// +required
	ExtendedResources v1.ResourceList `json:"extendedResources"`
}

// InstanceTypeExtensionSpec is the top level instance type extension specification.
type InstanceTypeExtensionSpec struct {
	// +required
	Extensions []Extension `json:"extensions"`
}

// InstanceTypeExtension is the Schema for the InstanceTypeExtension API
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=extensions,scope=Cluster,categories=karpenter
type InstanceTypeExtension struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec InstanceTypeExtensionSpec `json:"spec"`
}

// InstanceTypeExtensionList contains a list of InstanceTypeExtension
// +kubebuilder:object:root=true
type InstanceTypeExtensionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InstanceTypeExtension `json:"items"`
}
