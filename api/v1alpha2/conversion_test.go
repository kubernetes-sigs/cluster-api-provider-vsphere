package v1alpha2

import (
	"testing"

	utilconversion "sigs.k8s.io/cluster-api/util/conversion"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha3"
)

func TestFuzzyConversion(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(AddToScheme(scheme)).To(Succeed())
	g.Expect(v1alpha3.AddToScheme(scheme)).To(Succeed())

	t.Run("for VSphereCluster", utilconversion.FuzzTestFunc(scheme, &v1alpha3.VSphereCluster{}, &VSphereCluster{}))
	t.Run("for VSphereMachine", utilconversion.FuzzTestFunc(scheme, &v1alpha3.VSphereMachine{}, &VSphereMachine{}))
	t.Run("for VSphereMachineTemplate", utilconversion.FuzzTestFunc(scheme, &v1alpha3.VSphereMachineTemplate{}, &VSphereMachineTemplate{}))
}
