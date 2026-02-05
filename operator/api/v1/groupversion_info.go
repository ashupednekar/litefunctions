// +kubebuilder:object:generate=true
// +groupName=apps.ashupednekar.github.io
package v1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	GroupVersion  = schema.GroupVersion{Group: "apps.ashupednekar.github.io", Version: "v1"}
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}
	AddToScheme   = SchemeBuilder.AddToScheme
)

func init() {
	SchemeBuilder.Register(&Function{}, &FunctionList{})
}
