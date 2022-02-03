/*
Copyright 2022.

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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ManagedClusterSpec defines the desired state of ManagedCluster
type ManagedClusterSpec struct {
	// ManagedClusterClientConfigs represents a list of the apiserver address of the managed cluster.
	// If it is empty, the managed cluster has no accessible address for the hub to connect with it.
	// +optional
	ManagedClusterClientConfigs []ClientConfig `json:"managedClusterClientConfigs,omitempty"`

	// hubAcceptsClient represents that hub accepts the joining of Klusterlet agent on
	// the managed cluster with the hub. The default value is false, and can only be set
	// true when the user on hub has an RBAC rule to UPDATE on the virtual subresource
	// of managedclusters/accept.
	// When the value is set true, a namespace whose name is the same as the name of ManagedCluster
	// is created on the hub. This namespace represents the managed cluster, also role/rolebinding is created on
	// the namespace to grant the permision of access from the agent on the managed cluster.
	// When the value is set to false, the namespace representing the managed cluster is
	// deleted.
	// +required
	HubAcceptsClient bool `json:"hubAcceptsClient"`

	// LeaseDurationSeconds is used to coordinate the lease update time of Klusterlet agents on the managed cluster.
	// If its value is zero, the Klusterlet agent will update its lease every 60 seconds by default
	// +optional
	// +kubebuilder:default=60
	LeaseDurationSeconds int32 `json:"leaseDurationSeconds,omitempty"`

	// Taints is a property of managed cluster that allow the cluster to be repelled when scheduling.
	// Taints, including 'ManagedClusterUnavailable' and 'ManagedClusterUnreachable', can not be added/removed by agent
	// running on the managed cluster; while it's fine to add/remove other taints from either hub cluser or managed cluster.
	// +optional
	Taints []Taint `json:"taints,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ManagedCluster is the Schema for the managedclusters API
type ManagedCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ManagedClusterSpec   `json:"spec,omitempty"`
	Status ManagedClusterStatus `json:"status,omitempty"`
	Lease  MemberClusterLease   `json:"lease,omitempty"`
}

// ClientConfig represents the apiserver address of the managed cluster.
type ClientConfig struct {
	// URL is the URL of apiserver endpoint of the managed cluster.
	// +required
	URL string `json:"url"`

	// CABundle is the ca bundle to connect to apiserver of the managed cluster.
	// System certs are used if it is not set.
	// +optional
	CABundle []byte `json:"caBundle,omitempty"`

	// SecretRef This will make sure that the secret we are looking at is a correct [Secret]
	SecretRef string `json:"secretRef"`
}

// The managed cluster this Taint is attached to has the "effect" on
// any placement that does not tolerate the Taint.
type Taint struct {
	// Key is the taint key applied to a cluster. e.g. bar or foo.example.com/bar.
	// The regex it matches is (dns1123SubdomainFmt/)?(qualifiedNameFmt)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*/)?(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])$`
	// +kubebuilder:validation:MaxLength=316
	// +required
	Key string `json:"key"`
	// Value is the taint value corresponding to the taint key.
	// +kubebuilder:validation:MaxLength=1024
	// +optional
	Value string `json:"value,omitempty"`
	// Effect indicates the effect of the taint on placements that do not tolerate the taint.
	// Valid effects are NoSelect, PreferNoSelect and NoSelectIfNew.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum:=NoSelect;PreferNoSelect;NoSelectIfNew
	// +required
	Effect TaintEffect `json:"effect"`
	// TimeAdded represents the time at which the taint was added.
	// +nullable
	// +required
	TimeAdded metav1.Time `json:"timeAdded"`
}

type TaintEffect string

const (
	// TaintEffectNoSelect means placements are not allowed to select the cluster unless they tolerate the taint.
	// The cluster will be removed from the placement cluster decisions if a placement has already selected
	// this cluster.
	TaintEffectNoSelect TaintEffect = "NoSelect"
	// TaintEffectPreferNoSelect means the scheduler tries not to select the cluster, rather than prohibiting
	// placements from selecting the cluster entirely.
	TaintEffectPreferNoSelect TaintEffect = "PreferNoSelect"
	// TaintEffectNoSelectIfNew means placements are not allowed to select the cluster unless
	// 1) they tolerate the taint;
	// 2) they have already had the cluster in their cluster decisions;
	TaintEffectNoSelectIfNew TaintEffect = "NoSelectIfNew"
)

const (
	// ManagedClusterTaintUnavailable is the key of the taint added to a managed cluster when it is not available.
	// To be specific, the cluster has a condtion 'ManagedClusterConditionAvailable' with status of 'False';
	ManagedClusterTaintUnavailable string = "cluster.aks-caravel.mcm/unavailable"
	// ManagedClusterTaintUnreachable is the key of the taint added to a managed cluster when it is not reachable.
	// To be specific,
	// 1) The cluster has no condition 'ManagedClusterConditionAvailable';
	// 2) Or the status of condtion 'ManagedClusterConditionAvailable' is 'Unknown';
	ManagedClusterTaintUnreachable string = "cluster.aks-caravel.mcm/unreachable"
)

// ManagedClusterStatus represents the current status of joined managed cluster.
type ManagedClusterStatus struct {
	// Conditions contains the different condition statuses for this managed cluster.
	Conditions []metav1.Condition `json:"conditions"`

	// Capacity represents the total resource capacity from all nodeStatuses
	// on the managed cluster.
	Capacity ResourceList `json:"capacity,omitempty"`

	// Allocatable represents the total allocatable resources on the managed cluster.
	Allocatable ResourceList `json:"allocatable,omitempty"`

	// Version represents the kubernetes version of the managed cluster.
	Version ManagedClusterVersion `json:"version,omitempty"`

	// ClusterClaims represents cluster information that a managed cluster claims,
	// for example a unique cluster identifier (id.k8s.io) and kubernetes version
	// (kubeversion.aks-caravel.mcm). They are written from the managed
	// cluster. The set of claims is not uniform across a fleet, some claims can be
	// vendor or version specific and may not be included from all managed clusters.
	// +optional
	ClusterClaims []ManagedClusterClaim `json:"clusterClaims,omitempty"`
}

// ManagedClusterVersion represents version information about the managed cluster.
type ManagedClusterVersion struct {
	// Kubernetes is the kubernetes version of managed cluster.
	// +optional
	Kubernetes string `json:"kubernetes,omitempty"`
}

// ManagedClusterClaim represents a ClusterClaim collected from a managed cluster.
type ManagedClusterClaim struct {
	// Name is the name of a ClusterClaim resource on managed cluster. It's a well known
	// or customized name to identify the claim.
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name,omitempty"`

	// Value is a claim-dependent string
	// +kubebuilder:validation:MaxLength=1024
	// +kubebuilder:validation:MinLength=1
	Value string `json:"value,omitempty"`
}

const (
	// ManagedClusterConditionJoined means the managed cluster has successfully joined the hub.
	ManagedClusterConditionJoined string = "ManagedClusterJoined"
	// ManagedClusterConditionHubAccepted means the request to join the cluster is
	// approved by cluster-admin on hub.
	ManagedClusterConditionHubAccepted string = "HubAcceptedManagedCluster"
	// ManagedClusterConditionHubDenied means the request to join the cluster is denied by
	// cluster-admin on hub.
	ManagedClusterConditionHubDenied string = "HubDeniedManagedCluster"
	// ManagedClusterConditionAvailable means the managed cluster is available. If a managed
	// cluster is available, the kube-apiserver is healthy and the Klusterlet agent is
	// running with the minimum deployment on this managed cluster
	ManagedClusterConditionAvailable string = "ManagedClusterConditionAvailable"
)

// ResourceName is the name identifying various resources in a ResourceList.
type ResourceName string

const (
	// ResourceCPU defines the number of CPUs in cores. (500m = .5 cores)
	ResourceCPU ResourceName = "cpu"
	// ResourceMemory defines the amount of memory in bytes. (500Gi = 500GiB = 500 * 1024 * 1024 * 1024)
	ResourceMemory ResourceName = "memory"
)

// ResourceList defines a map for the quantity of different resources, the definition
// matches the ResourceList defined in k8s.io/api/core/v1.
type ResourceList map[ResourceName]resource.Quantity

//+kubebuilder:object:root=true

// ManagedClusterList contains a list of ManagedCluster
type ManagedClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ManagedCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ManagedCluster{}, &ManagedClusterList{})
}
