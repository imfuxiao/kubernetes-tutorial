package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Etcd struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EtcdSpec   `json:"spec"`
	Status EtcdStatus `json:"status"`
}

type EtcdSpec struct {
	Name          string `json:"name"`
	Namespace     string `json:"namespace"`
	DataDir       string `json:"datDir"`
	SnapshotCount int    `json:"snapshotCount"`
	Image         string `json:"image"`
}

type EtcdStatus struct {
	corev1.PodStatus `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type EtcdList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Etcd `json:"items"`
}
