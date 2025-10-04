package v1alpha1

import (
	"fmt"
	"testing"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

func TestVMOperatorDependencies_SetVCenterFromVCenterSimulator(t *testing.T) {
	type fields struct {
		TypeMeta   v1.TypeMeta
		ObjectMeta v1.ObjectMeta
		Spec       VMOperatorDependenciesSpec
		Status     VMOperatorDependenciesStatus
	}
	type args struct {
		vCenterSimulator *VCenterSimulator
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			args: args{
				vCenterSimulator: &VCenterSimulator{
					Status: VCenterSimulatorStatus{
						Host:       "Host",
						Username:   "Username",
						Password:   "Password",
						Thumbprint: "Thumbprint",
					},
				},
			},
			fields: fields{
				TypeMeta: v1.TypeMeta{
					Kind:       "VMOperatorDependencies",
					APIVersion: GroupVersion.String(),
				},
				ObjectMeta: v1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec:   VMOperatorDependenciesSpec{},
				Status: VMOperatorDependenciesStatus{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &VMOperatorDependencies{
				TypeMeta:   tt.fields.TypeMeta,
				ObjectMeta: tt.fields.ObjectMeta,
				Spec:       tt.fields.Spec,
				Status:     tt.fields.Status,
			}
			d.SetVCenterFromVCenterSimulator(tt.args.vCenterSimulator)

			foo, _ := yaml.Marshal(d)
			fmt.Println(string(foo))
		})
	}
}
