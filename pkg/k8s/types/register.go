package types

import (
	"github.com/kubemq-io/kubemqctl/pkg/k8s/types/kubemqcluster"
	"github.com/kubemq-io/kubemqctl/pkg/k8s/types/kubemqconnector"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	GroupName    = "core.k8s.kubemq.io"
	GroupVersion = "v1beta1"
)

var SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: GroupVersion}

var (
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme   = SchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&kubemqcluster.KubemqCluster{},
		&kubemqcluster.KubemqClusterList{},
		&kubemqconnector.KubemqConnector{},
		&kubemqconnector.KubemqConnectorList{},
	)

	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}
