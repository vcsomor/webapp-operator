/*
Copyright 2023.

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
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// WebappSpec defines the desired state of Webapp
type WebappSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Replicas defines the number of Webapp instances
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +kubebuilder:default=3
	// +kubebuilder:validation:Optional
	Replicas int32 `json:"replicas,omitempty"`

	// Host defines the host that the Web application will listen on
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Host string `json:"host,omitempty"`

	// Image defines the image for the webapp deployment
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +kubebuilder:default="nginx:latest"
	// +kubebuilder:validation:Optional
	Image string `json:"image,omitempty"`

	// ContainerPort defines the port that will be used to init the container with the image
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +kubebuilder:default=80
	// +kubebuilder:validation:Optional
	ContainerPort int32 `json:"containerPort,omitempty"`

	// CaProviderUrl is the URL account for issuing tls certificates
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	CertManager string `json:"cert-manager,omitempty"`
}

// WebappStatus defines the observed state of Webapp
type WebappStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Webapp is the Schema for the webapps API
// +kubebuilder:subresource:status
type Webapp struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WebappSpec   `json:"spec,omitempty"`
	Status WebappStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// WebappList contains a list of Webapp
type WebappList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Webapp `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Webapp{}, &WebappList{})
}
