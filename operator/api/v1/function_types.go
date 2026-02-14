package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

type FunctionSpec struct {
	IsActive        bool   `json:"isActive"`
	IsAsync         bool   `json:"isAsync"`
	DeProvisionTime string `json:"deProvisionTime"`
	Language        string `json:"language"`
	Name            string `json:"name"`
	Method          string `json:"method"`
	Project         string `json:"project"`
	GitCreds        string `json:"git_creds"`
}

type FunctionStatus struct {
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
type Function struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   FunctionSpec   `json:"spec"`
	Status FunctionStatus `json:"status,omitempty"`
}

func (f *Function) DeepCopyObject() runtime.Object {
	if c := f.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (f *Function) DeepCopy() *Function {
	if f == nil {
		return nil
	}
	out := new(Function)
	f.DeepCopyInto(out)
	return out
}

func (f *Function) DeepCopyInto(out *Function) {
	*out = *f
	out.TypeMeta = f.TypeMeta
	f.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
}

// +kubebuilder:object:root=true
type FunctionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Function `json:"items"`
}

func (l *FunctionList) DeepCopyObject() runtime.Object {
	if c := l.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (l *FunctionList) DeepCopy() *FunctionList {
	if l == nil {
		return nil
	}
	out := new(FunctionList)
	l.DeepCopyInto(out)
	return out
}

func (l *FunctionList) DeepCopyInto(out *FunctionList) {
	*out = *l
	out.TypeMeta = l.TypeMeta
	l.ListMeta.DeepCopyInto(&out.ListMeta)
	if l.Items != nil {
		in, out := &l.Items, &out.Items
		*out = make([]Function, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}
