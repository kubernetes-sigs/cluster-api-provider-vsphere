package v1alpha3

import (
	"k8s.io/apimachinery/pkg/conversion"
	"sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha4"
)

func Convert_v1alpha3_VSphereClusterSpec_To_v1alpha4_VSphereClusterSpec(in *VSphereClusterSpec, out *v1alpha4.VSphereClusterSpec, s conversion.Scope) error { //nolint
	return autoConvert_v1alpha3_VSphereClusterSpec_To_v1alpha4_VSphereClusterSpec(in, out, s)
}
