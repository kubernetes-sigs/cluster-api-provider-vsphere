package govmomi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"time"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	vsphereconfigv1 "sigs.k8s.io/cluster-api-provider-vsphere/pkg/apis/vsphereproviderconfig/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/constants"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	clusterv1alpha1 "sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset/typed/cluster/v1alpha1"
	"sigs.k8s.io/yaml"
)

type GovmomiMachine struct {
	s             *SessionContext
	k8sClient     kubernetes.Interface
	clusterAPI    clusterv1alpha1.ClusterV1alpha1Interface
	cluster       *clusterv1.Cluster
	machine       *clusterv1.Machine
	config        *vsphereconfigv1.VsphereMachineProviderConfig
	status        *vsphereconfigv1.VsphereMachineProviderStatus
	eventRecorder record.EventRecorder
	Name          string
}

func NewGovmomiMachine(cluster *clusterv1.Cluster, machine *clusterv1.Machine, s *SessionContext, eventRecorder record.EventRecorder, k8sClient kubernetes.Interface, clusterAPI clusterv1alpha1.ClusterV1alpha1Interface) (*GovmomiMachine, error) {

	config := &vsphereconfigv1.VsphereMachineProviderConfig{}
	if machine.Spec.ProviderSpec.Value == nil {
		return nil, fmt.Errorf("machine providerconfig is invalid (nil)")
	}

	err := yaml.Unmarshal(machine.Spec.ProviderSpec.Value.Raw, config)
	if err != nil {
		return nil, fmt.Errorf("machine providerconfig unmarshalling failure: %s", err)
	}

	status := &vsphereconfigv1.VsphereMachineProviderStatus{}
	if machine.Status.ProviderStatus != nil {
		err := json.Unmarshal(machine.Status.ProviderStatus.Raw, status)
		if err != nil {
			return nil, fmt.Errorf("error unmarshaling machine provider status: %s", err)
		}
	}

	// fail fast on any cluster level config issues, we can then safely ignore parsing errors returned in the future
	_, err = newGovmomiCluster(cluster)
	if err != nil {
		return nil, err
	}
	return &GovmomiMachine{
		s:             s,
		cluster:       cluster,
		machine:       machine,
		status:        status,
		config:        config,
		eventRecorder: eventRecorder,
		k8sClient:     k8sClient,
		clusterAPI:    clusterAPI,
		Name:          machine.Name,
	}, nil
}

func (m *GovmomiMachine) IsExists(ctx context.Context) bool {
	var vm mo.VirtualMachine
	err := m.s.session.RetrieveOne(ctx, m.GetMOF(), []string{"name"}, &vm)
	return err == nil
}

func (m *GovmomiMachine) GetCluster() *GovmomiCluster {
	cluster, _ := newGovmomiCluster(m.cluster)
	cluster.s = m.s
	cluster.k8sClient = m.k8sClient
	return cluster
}

func (m *GovmomiMachine) GetActiveTasks() {

}

func (m *GovmomiMachine) GetVM(ctx context.Context) (*object.VirtualMachine, error) {
	return object.NewVirtualMachine(m.s.session.Client, m.GetMOF()), nil
}

func (m *GovmomiMachine) GetIP() (string, error) {
	if m.machine.ObjectMeta.Annotations == nil {
		return "", errors.New("could not get IP")
	}
	if ip, ok := m.machine.ObjectMeta.Annotations[constants.VmIpAnnotationKey]; ok && ip != "" {
		return ip, nil
	}
	return "", errors.New("could not get IP")
}

func (m *GovmomiMachine) WaitForIP(ctx context.Context) (string, error) {
	return "", nil
}

func (m *GovmomiMachine) Eventf(name string, message string, args ...interface{}) {
	klog.V(4).Infof(fmt.Sprintf("[%s] %s: ", m.machine.Name, name) + fmt.Sprintf(message, args...))
	m.eventRecorder.Eventf(m.machine, corev1.EventTypeNormal, name, message, args...)
}

func (m *GovmomiMachine) GetMOF() types.ManagedObjectReference {
	return types.ManagedObjectReference{
		Type:  "VirtualMachine",
		Value: m.config.MachineRef,
	}
}

func (m *GovmomiMachine) IsPoweredOn(ctx context.Context) (bool, error) {
	var vm mo.VirtualMachine
	err := m.s.session.RetrieveOne(ctx, m.GetMOF(), []string{"name", "runtime.powerState"}, &vm)
	if err != nil {
		return false, err
	}
	return vm.Runtime.PowerState == types.VirtualMachinePowerStatePoweredOn, nil

	return false, nil
}

func (m *GovmomiMachine) PowerOn(ctx context.Context) *Task {
	vm, err := m.GetVM(ctx)
	if err != nil {
		return &Task{err: err, ctx: ctx}
	}

	task, err := vm.PowerOn(ctx)
	return &Task{
		err:  err,
		task: task,
		ctx:  ctx,
	}
}

func (m *GovmomiMachine) Delete(ctx context.Context) *Task {
	vm, err := m.GetVM(ctx)
	if err != nil {
		return &Task{err: err, ctx: ctx}
	}
	task, err := vm.Destroy(ctx)
	return &Task{
		err:  err,
		task: task,
		ctx:  ctx,
	}
}

func (m *GovmomiMachine) PowerOff(ctx context.Context) *Task {
	vm, err := m.GetVM(ctx)
	if err != nil {
		return &Task{err: err, ctx: ctx}
	}
	task, err := vm.PowerOff(ctx)
	return &Task{
		err:  err,
		task: task,
		ctx:  ctx,
	}
}

func (m *GovmomiMachine) UpdateVMReference(vmref string) error {
	providerSpec := m.config
	providerSpec.MachineRef = vmref
	// Set the Kind and APIVersion again since they are not returned
	// See the following Issues for details:
	// https://github.com/kubernetes/client-go/issues/308
	// https://github.com/kubernetes/kubernetes/issues/3030
	providerSpec.Kind = reflect.TypeOf(*providerSpec).Name()
	providerSpec.APIVersion = vsphereconfigv1.SchemeGroupVersion.String()
	newMachine := m.machine.DeepCopy()
	out, err := json.Marshal(providerSpec)
	if err != nil {
		return fmt.Errorf("Error marshaling ProviderConfig: %s", err)
	}
	newMachine.Spec.ProviderSpec.Value = &runtime.RawExtension{Raw: out}
	newMachine, err = m.clusterAPI.Machines(newMachine.Namespace).Update(newMachine)
	if err != nil {
		return fmt.Errorf("Error in updating the machine ref: %s", err)
	}

	// This is needed otherwise the update status on the original machine object would fail as the resource has been updated by the previous call
	// Note: We are not mutating the object retrieved from the informer ever. The updatedmachine is the updated resource generated using DeepCopy
	// This would just update the reference to be the newer object so that the status update works
	m.machine = newMachine
	m.config = providerSpec
	return nil
}

func (m *GovmomiMachine) SetTaskRef(taskref string) error {
	oldProviderStatus := m.status

	if oldProviderStatus != nil && oldProviderStatus.TaskRef == taskref {
		// Nothing to update
		return nil
	}
	newProviderStatus := &vsphereconfigv1.VsphereMachineProviderStatus{}
	// create a copy of the old status so that any other fields except the ones we want to change can be retained
	if oldProviderStatus != nil {
		newProviderStatus = oldProviderStatus.DeepCopy()
	}
	newProviderStatus.TaskRef = taskref
	newProviderStatus.LastUpdated = time.Now().UTC().String()
	out, err := json.Marshal(newProviderStatus)
	newMachine := m.machine.DeepCopy()
	newMachine.Status.ProviderStatus = &runtime.RawExtension{Raw: out}
	if m.clusterAPI == nil { // TODO: currently supporting nil for testing
		return nil
	}
	_, err = m.clusterAPI.Machines(newMachine.Namespace).UpdateStatus(newMachine)
	if err != nil {
		return fmt.Errorf("Error in updating the machine ref: %s", err)
	}
	return nil
}

func (m *GovmomiMachine) UpdateAnnotations(annotations map[string]string) error {
	nmachine := m.machine.DeepCopy()
	if nmachine.ObjectMeta.Annotations == nil {
		nmachine.ObjectMeta.Annotations = make(map[string]string)
	}
	for k, v := range annotations {
		nmachine.ObjectMeta.Annotations[k] = v
	}
	_, err := m.clusterAPI.Machines(nmachine.Namespace).Update(nmachine)
	if err != nil {
		return err
	}
	return nil
}

func (m *GovmomiMachine) GetKubeConfig() (string, error) {
	ip, err := m.GetIP()
	if err != nil {
		klog.Info("cannot get kubeconfig because found no IP")
		return "", err
	}
	klog.Infof("pulling kubeconfig (using ssh) from %s", ip)
	var out bytes.Buffer
	cmd := exec.Command(
		"ssh", "-i", "~/.ssh/vsphere_tmp",
		"-q",
		"-o", "StrictHostKeyChecking no",
		"-o", "UserKnownHostsFile /dev/null",
		fmt.Sprintf("ubuntu@%s", ip),
		"sudo cat /etc/kubernetes/admin.conf")
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		klog.Infof("ssh failed with error = %s", err.Error())
	}
	result := strings.TrimSpace(out.String())
	if len(result) > 0 {
		klog.Info("ssh pulled kubeconfig")
	}
	return result, err
}
