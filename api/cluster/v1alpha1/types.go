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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:object:root=true
// MemberClusterLeaseSpec defines the desired state of MemberClusterLease
type MemberClusterLeaseSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// the unique ID of the fleet this member cluster belongs to
	FleetID string `json:"fleetID"`

	// LastLeaseRenewTime is the last time the hub cluster controller can reach the member cluster
	LastLeaseRenewTime metav1.Time `json:"lastLeaseRenewTime"`

	// LastJoinTime is the last time the hub cluster controller re-establish connection to the member cluster
	LastJoinTime metav1.Time `json:"lastJoinTime"`
}

// MemberClusterLeaseStatus defines the observed state of MemberClusterLease
type MemberClusterLeaseStatus struct {
	// TODO: INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// MemberClusterLease is the Schema for the memberclusterleases API
type MemberClusterLease struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MemberClusterLeaseSpec   `json:"spec,omitempty"`
	Status MemberClusterLeaseStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MemberClusterLeaseList contains a list of MemberClusterLease
type MemberClusterLeaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MemberClusterLease `json:"items"`
}
