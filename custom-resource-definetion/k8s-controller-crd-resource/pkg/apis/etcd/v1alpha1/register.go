package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	GroupName = "crd.fuxiao.dev"
	Version   = "v1alpha1"
)

var (
	SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: Version}
)

var (
	SchemaBuilder = runtime.NewSchemeBuilder(AddToScheme)
)

func AddToScheme(schema *runtime.Scheme) error {
	schema.AddKnownTypes(SchemeGroupVersion, &Etcd{}, &EtcdList{})

	// register etcd etcdList type in the scheme
	metav1.AddToGroupVersion(schema, SchemeGroupVersion)
	return nil
}

func Kind(kind string) schema.GroupKind {
	return SchemeGroupVersion.WithKind(kind).GroupKind()
}

func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}
