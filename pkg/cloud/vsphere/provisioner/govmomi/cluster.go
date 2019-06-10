package govmomi

import (
	"context"
	"net/url"
	"time"

	"github.com/google/uuid"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"

	"encoding/json"

	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog"
	vsphereconfigv1 "sigs.k8s.io/cluster-api-provider-vsphere/pkg/apis/vsphereproviderconfig/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/constants"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	clusterv1alpha1 "sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset/typed/cluster/v1alpha1"
)

type GovmomiCluster struct {
	clusterAPI clusterv1alpha1.ClusterV1alpha1Interface
	cluster    *clusterv1.Cluster
	config     *vsphereconfigv1.VsphereClusterProviderConfig
	status     *vsphereconfigv1.VsphereClusterProviderStatus
	k8sClient  kubernetes.Interface
	s          *SessionContext
}

func newGovmomiCluster(cluster *clusterv1.Cluster) (*GovmomiCluster, error) {
	config := &vsphereconfigv1.VsphereClusterProviderConfig{}
	status := &vsphereconfigv1.VsphereClusterProviderStatus{}
	if cluster.Spec.ProviderSpec.Value == nil {
		return nil, fmt.Errorf("cluster providerconfig is invalid (nil)")
	}

	err := yaml.Unmarshal(cluster.Spec.ProviderSpec.Value.Raw, config)
	if err != nil {
		return nil, fmt.Errorf("cluster providerconfig unmarshalling failure: %s", err)
	}

	if cluster.Status.ProviderStatus != nil {
		err = json.Unmarshal(cluster.Status.ProviderStatus.Raw, status)
		if err != nil {
			return nil, err
		}
	}
	return &GovmomiCluster{
		status:  status,
		config:  config,
		cluster: cluster,
	}, nil
}

func (c *GovmomiCluster) GetTask(ctx context.Context, ref string) *mo.Task {
	var taskmo mo.Task
	taskref := types.ManagedObjectReference{
		Type:  "Task",
		Value: ref,
	}
	err := c.s.session.RetrieveOne(ctx, taskref, []string{"info"}, &taskmo)
	if err != nil {
		return nil
	}
	return &taskmo
}

func (c *GovmomiCluster) findVMByInstanceUUID(ctx context.Context, uuid string) (string, error) {
	klog.V(6).Infof("[find] Trying to check existence of the VM via InstanceUUID %s", uuid)
	si := object.NewSearchIndex(c.s.session.Client)
	instanceUUID := true
	vmRef, err := si.FindByUuid(ctx, nil, string(uuid), true, &instanceUUID)
	if err != nil {
		return "", fmt.Errorf("error quering virtual machine or template using FindByUuid: %s", err)
	}
	if vmRef != nil {
		klog.V(4).Infof("[find] lookup uuid %s -> ref: %s", uuid, vmRef.Reference().Value)
		return vmRef.Reference().Value, nil
	}
	return "", nil
}

func isValidUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}

func (c *GovmomiCluster) FindVM(ctx context.Context, dc *object.Datacenter, nameOrUid string) (*object.VirtualMachine, error) {
	// Let's check to make sure we can find the template earlier on... Plus, we need
	// the cluster/host info if we want to deploy direct to the cluster/host.
	var src *object.VirtualMachine
	var err error

	if isValidUUID(nameOrUid) {
		// If the passed VMTemplate is a valid UUID, then first try to find it treating that as InstanceUUID
		// In case if are not able to locate a matching VM then fall back to searching using the VMTemplate
		// as a name
		klog.V(4).Infof("Trying to resolve the VMTemplate as InstanceUUID %s", nameOrUid)
		si := object.NewSearchIndex(c.s.session.Client)
		instanceUUID := true
		templateref, err := si.FindByUuid(ctx, dc, nameOrUid, true, &instanceUUID)
		if err != nil {
			return nil, fmt.Errorf("error querying virtual machine or template using FindByUuid: %s", err)
		}
		if templateref != nil {
			src = object.NewVirtualMachine(c.s.session.Client, templateref.Reference())
		}
	}
	if src == nil {
		klog.V(4).Infof("Trying to resolve the VMTemplate as Name %s", nameOrUid)
		src, err = c.s.finder.VirtualMachine(ctx, nameOrUid)
		if err != nil {
			return nil, fmt.Errorf("VirtualMachine finder failed. err=%s", err)
		}
	}
	return src, nil
}

func (c *GovmomiCluster) GetVirtualMachineMO(vm *object.VirtualMachine) (*mo.VirtualMachine, error) {
	klog.V(6).Infof("[get] Fetching properties for VM %q", vm.InventoryPath)
	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultAPITimeout)
	defer cancel()
	var props mo.VirtualMachine
	if err := vm.Properties(ctx, vm.Reference(), nil, &props); err != nil {
		return nil, err
	}
	return &props, nil
}

func (c *GovmomiCluster) GetHostMO(host *object.HostSystem) (*mo.HostSystem, error) {
	klog.V(6).Infof("[DEBUG] Fetching properties for host %q", host.InventoryPath)
	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultAPITimeout)
	defer cancel()
	var props mo.HostSystem
	if err := host.Properties(ctx, host.Reference(), nil, &props); err != nil {
		return nil, err
	}
	return &props, nil
}

func (c *GovmomiCluster) Touch() error {
	status := &vsphereconfigv1.VsphereClusterProviderStatus{LastUpdated: time.Now().UTC().String()}
	out, err := json.Marshal(status)
	ncluster := c.cluster.DeepCopy()
	ncluster.Status.ProviderStatus = &runtime.RawExtension{Raw: out}
	_, err = c.clusterAPI.Clusters(ncluster.Namespace).UpdateStatus(ncluster)
	return err

}

func (c *GovmomiCluster) GetHost() string {
	// cloud provider requires bare IP:port, so if it is parseable as a url with a scheme, then
	// strip the scheme and path.  Otherwise continue.
	// TODO replace with better input validation.
	serverURL, err := url.Parse(c.config.VsphereServer)
	if err == nil && serverURL.Host != "" {
		return serverURL.Host
	}
	return c.config.VsphereServer
}

func (c *GovmomiCluster) GetCredentials() (username string, password string, err error) {
	// If the vsphereCredentialSecret is specified then read that secret to get the credentials
	if c.config.VsphereCredentialSecret != "" {
		klog.V(4).Infof("Fetching vsphere credentials from secret %s", c.config.VsphereCredentialSecret)
		secret, err := c.k8sClient.Core().Secrets(c.cluster.Namespace).Get(c.config.VsphereCredentialSecret, metav1.GetOptions{})
		if err != nil {
			return "", "", fmt.Errorf("Error reading secret %s", c.config.VsphereCredentialSecret)
		}
		if username, ok := secret.Data[constants.VsphereUserKey]; ok {
			if password, ok := secret.Data[constants.VspherePasswordKey]; ok {
				return string(username), string(password), nil
			}
		}
		return "", "", fmt.Errorf("Improper secret: Secret %s should have the keys `%s` and `%s` defined in it", c.config.VsphereCredentialSecret, constants.VsphereUserKey, constants.VspherePasswordKey)
	}
	return c.config.VsphereUser, c.config.VspherePassword, nil
}
