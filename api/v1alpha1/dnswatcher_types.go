/*
Copyright 2025 Thomas Boerger <thomas@webhippie.de>.

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

// PathSpec defines a JSONPath into DNS record type mapping.
type PathSpec struct {
	// Path is a JSONPath expression evaluated against the target resource (e.g. "$.status.ip").
	// +kubebuilder:validation:MinLength=1
	Path string `json:"path"`

	// Type is the DNS record type (A, AAAA, CNAME, TXT, etc.).
	// +kubebuilder:validation:MinLength=1
	Type string `json:"type"`
}

// DNSWatcherSpec defines the desired state of DNSWatcher.
type DNSWatcherSpec struct {
	// Group is the API group of the resources to watch.
	// Use an empty string for core Kubernetes resources (e.g. ConfigMap, Pod).
	// +optional
	Group string `json:"group"`

	// Version is the API version of the resources to watch (e.g. "v1beta1").
	// +kubebuilder:validation:MinLength=1
	Version string `json:"version"`

	// Kind is the kind of the resources to watch (e.g. "HetznerCluster").
	// +kubebuilder:validation:MinLength=1
	Kind string `json:"kind"`

	// Namespace optionally restricts which namespace of target resources to watch.
	// When empty, all namespaces are watched. Target resources must be namespaced.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// RecordTemplate is a Go template rendered to produce the DNS record name.
	// Available template variables: .Name (resource name), .Namespace (resource namespace).
	// Example: "{{ .Name }}-control-plane.example.com"
	// +kubebuilder:validation:MinLength=1
	RecordTemplate string `json:"recordTemplate"`

	// Paths defines one or more JSONPath expressions to extract target values from the watched resource.
	// +kubebuilder:validation:MinItems=1
	Paths []PathSpec `json:"paths"`
}

// DNSWatcherStatus defines the observed state of DNSWatcher.
type DNSWatcherStatus struct {
	// Conditions reflect the current state of the DNSWatcher.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// ObservedGeneration is the last generation that was reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// WatchedCount is the number of target resource instances currently watched.
	// +optional
	WatchedCount int `json:"watchedCount,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=dw,categories=external-dns
// +kubebuilder:printcolumn:name="Group",type=string,JSONPath=`.spec.group`
// +kubebuilder:printcolumn:name="Kind",type=string,JSONPath=`.spec.kind`
// +kubebuilder:printcolumn:name="Watched",type=integer,JSONPath=`.status.watchedCount`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// DNSWatcher defines a rule for watching Kubernetes resources and creating
// ExternalDNS DNSEndpoint resources from extracted IP addresses or hostnames.
type DNSWatcher struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DNSWatcherSpec   `json:"spec,omitempty"`
	Status DNSWatcherStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DNSWatcherList contains a list of DNSWatcher.
type DNSWatcherList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DNSWatcher `json:"items"`
}
