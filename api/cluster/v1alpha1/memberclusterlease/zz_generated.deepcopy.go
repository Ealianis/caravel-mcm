//go:build !ignore_autogenerated
// +build !ignore_autogenerated

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

// Code generated by controller-gen. DO NOT EDIT.

package memberclusterlease

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MemberClusterLease) DeepCopyInto(out *MemberClusterLease) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MemberClusterLease.
func (in *MemberClusterLease) DeepCopy() *MemberClusterLease {
	if in == nil {
		return nil
	}
	out := new(MemberClusterLease)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *MemberClusterLease) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MemberClusterLeaseList) DeepCopyInto(out *MemberClusterLeaseList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]MemberClusterLease, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MemberClusterLeaseList.
func (in *MemberClusterLeaseList) DeepCopy() *MemberClusterLeaseList {
	if in == nil {
		return nil
	}
	out := new(MemberClusterLeaseList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *MemberClusterLeaseList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MemberClusterLeaseSpec) DeepCopyInto(out *MemberClusterLeaseSpec) {
	*out = *in
	in.LastLeaseRenewTime.DeepCopyInto(&out.LastLeaseRenewTime)
	in.LastJoinTime.DeepCopyInto(&out.LastJoinTime)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MemberClusterLeaseSpec.
func (in *MemberClusterLeaseSpec) DeepCopy() *MemberClusterLeaseSpec {
	if in == nil {
		return nil
	}
	out := new(MemberClusterLeaseSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MemberClusterLeaseStatus) DeepCopyInto(out *MemberClusterLeaseStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MemberClusterLeaseStatus.
func (in *MemberClusterLeaseStatus) DeepCopy() *MemberClusterLeaseStatus {
	if in == nil {
		return nil
	}
	out := new(MemberClusterLeaseStatus)
	in.DeepCopyInto(out)
	return out
}
