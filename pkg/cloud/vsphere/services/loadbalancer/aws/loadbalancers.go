/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package aws

import (
	goctx "context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/elbv2/elbv2iface"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	infrav1 "sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha2"
	ctrclient "sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api-provider-vsphere/api/v1alpha2"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/constants"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services"
)

var (
	sessionCache sync.Map
)

func sessionForRegion(region string) (*session.Session, error) {
	s, ok := sessionCache.Load(region)
	if ok {
		return s.(*session.Session), nil
	}

	ns, err := session.NewSession(aws.NewConfig().WithRegion(region))
	if err != nil {
		return nil, err
	}

	sessionCache.Store(region, ns)
	return ns, nil
}

// Service is the AWS implementation of the vSphere LoadBalancer service and
// handles reconciling and deleting AWS elastic load balancers (ELBs) tied to
// vSphere clusters.
type Service struct {
	ELB         elbv2iface.ELBV2API
	awsLBConfig *v1alpha2.AWSLoadBalancerConfig
	client      ctrclient.Client
	logger      logr.Logger
}

// NewProvider returns a new instance of the AWS LoadBalancerService.
func NewProvider(configRef corev1.ObjectReference, client ctrclient.Client, logger logr.Logger) (services.LoadBalancerService, error) {
	elbClient, awsLBConfig, err := newAwsClient(configRef, client)
	if err != nil {
		return nil, err
	}
	return &Service{
		client:      client,
		logger:      logger,
		ELB:         elbClient,
		awsLBConfig: awsLBConfig,
	}, nil
}

// Reconcile ensures the AWS load balancer for a vSphere cluster is synchronized with the intended state.
func (svc *Service) Reconcile(loadBalancer *infrav1.LoadBalancer, machineIPs []string) (clusterv1.APIEndpoint, error) {
	targetGroupArn, err := svc.reconcileTargetGroup(loadBalancer, machineIPs)
	if err != nil {
		return clusterv1.APIEndpoint{}, err
	}

	loadBalancerArn, loadBalancerDNS, err := svc.reconcileLoadBalancer(loadBalancer.Name)
	if err != nil {
		return clusterv1.APIEndpoint{}, err
	}

	listenerPort, err := svc.reconcileListeners(loadBalancerArn, targetGroupArn, loadBalancer)
	if err != nil {
		return clusterv1.APIEndpoint{}, err
	}
	apiEndpoint := clusterv1.APIEndpoint{
		Host: *loadBalancerDNS,
		Port: int(*listenerPort),
	}
	return apiEndpoint, nil
}

func newAwsClient(configRef corev1.ObjectReference, client ctrclient.Client) (*elbv2.ELBV2, *v1alpha2.AWSLoadBalancerConfig, error) {
	ctx := goctx.Background()
	namespacedName := ctrclient.ObjectKey{
		Name:      configRef.Name,
		Namespace: configRef.Namespace,
	}
	awsLBConfig := &v1alpha2.AWSLoadBalancerConfig{}

	if err := client.Get(ctx, namespacedName, awsLBConfig); err != nil {
		return nil, nil, err
	}
	session, err := sessionForRegion(awsLBConfig.Spec.Region)
	if err != nil {
		return nil, nil, err
	}

	return elbv2.New(session), awsLBConfig, nil
}

func (svc *Service) reconcileTargetGroup(loadBalancer *infrav1.LoadBalancer, machineIPs []string) (*string, error) {
	var (
		// targetGroup is used to store the reconciled target group
		targetGroupOuput *elbv2.TargetGroup

		// describeTargetGroupsInput is used to get a list of the target groups
		describeTargetGroupsInput = &elbv2.DescribeTargetGroupsInput{
			Names: []*string{GetTargetGroupNameForCluster(loadBalancer.Name)},
		}

		// aws load balancer configuration
		awsLBConfig = svc.awsLBConfig

		// desiredTargetGroupInput describes the target group we want to find
		desiredTargetGroupInput = &elbv2.CreateTargetGroupInput{
			HealthCheckEnabled:         aws.Bool(true),
			HealthCheckIntervalSeconds: aws.Int64(int64(30 * time.Second.Seconds())),
			HealthCheckTimeoutSeconds:  aws.Int64(int64(10 * time.Second.Seconds())),
			HealthyThresholdCount:      aws.Int64(int64(3)),
			UnhealthyThresholdCount:    aws.Int64(int64(3)),
			Protocol:                   aws.String(elbv2.ProtocolEnumTcp),
			TargetType:                 aws.String(string("ip")),
			VpcId:                      &awsLBConfig.Spec.VpcID,
		}
	)
	if len(loadBalancer.Spec.Ports) == 0 {
		return nil, errors.New("empty load balancer ports")
	}

	// TODO yastij: handle multi-port listner/target support
	lbPort := loadBalancer.Spec.Ports[0]
	desiredTargetGroupInput.Name = &lbPort.Name
	if lbPort.TargetPort != nil {
		desiredTargetGroupInput.Port = aws.Int64(int64(*lbPort.TargetPort))
	} else {
		desiredTargetGroupInput.Port = aws.Int64(int64(lbPort.Port))
	}
	if lbPort.Protocol != nil {
		desiredTargetGroupInput.Protocol = aws.String(string(*lbPort.Protocol))
	}
	// Get a list of the target groups. If no target groups are discovered then
	// a new target group is created and its ARN is stored in targetGroupArn.
	describeTargetGroupsOutput, err := svc.ELB.DescribeTargetGroups(describeTargetGroupsInput)
	if err != nil {
		if IsNotFound(err) {
			createTargetGroupOutput, err := svc.ELB.CreateTargetGroup(desiredTargetGroupInput)
			if err != nil {
				return nil, err
			}
			targetGroupOuput = createTargetGroupOutput.TargetGroups[0]
		} else {
			return nil, err
		}
	}

	// If the target group already existed then make sure its assigned to
	// targetGroup. If the target group didn't already exist, then it was
	// assigned to targetGroupOutput when the target group was created above.
	if targetGroupOuput == nil {
		targetGroupOuput = describeTargetGroupsOutput.TargetGroups[0]
	}

	// Ensure the actual state of the target group matches the intended state of
	// target group. If the actual and intended states do not match then modify the
	// target group so its actual state matches the intended state.
	actualTargetGroupInput := convertTargetGroupOutputToInput(*targetGroupOuput)
	if !reflect.DeepEqual(actualTargetGroupInput, desiredTargetGroupInput) {
		modifyTargetGroup := &elbv2.ModifyTargetGroupInput{
			HealthCheckEnabled:         desiredTargetGroupInput.HealthCheckEnabled,
			HealthCheckIntervalSeconds: desiredTargetGroupInput.HealthCheckIntervalSeconds,
			HealthCheckTimeoutSeconds:  desiredTargetGroupInput.HealthCheckTimeoutSeconds,
			HealthyThresholdCount:      desiredTargetGroupInput.HealthyThresholdCount,
			UnhealthyThresholdCount:    desiredTargetGroupInput.UnhealthyThresholdCount,
		}
		if _, err := svc.ELB.ModifyTargetGroup(modifyTargetGroup); err != nil {
			return nil, err
		}
	}

	// If there is at least one control plane IP then register the IP addresses
	// to the target group.
	if len(machineIPs) > 0 {
		targetDescription := []*elbv2.TargetDescription{}
		for _, ipAddr := range machineIPs {
			targetDescription = append(targetDescription, &elbv2.TargetDescription{
				AvailabilityZone: aws.String(string("all")),
				Id:               aws.String(ipAddr),
				Port:             aws.Int64(int64(constants.DefaultBindPort)),
			})
		}
		registerTargetInput := &elbv2.RegisterTargetsInput{
			TargetGroupArn: targetGroupOuput.TargetGroupArn,
			Targets:        targetDescription,
		}
		if _, err := svc.ELB.RegisterTargets(registerTargetInput); err != nil {
			return nil, err
		}
	}
	svc.logger.V(6).Info("reconciled targetGroup", "targetGroupARN", targetGroupOuput.TargetGroupArn)

	return targetGroupOuput.TargetGroupArn, nil
}

// GetTargetGroupNameForCluster returns the name of a target group for the provided cluster.
func GetTargetGroupNameForCluster(clusterName string) *string {
	return aws.String(clusterName + "-targetgroup")
}

func convertTargetGroupOutputToInput(output elbv2.TargetGroup) *elbv2.CreateTargetGroupInput {
	return &elbv2.CreateTargetGroupInput{
		Name:                       output.TargetGroupName,
		HealthCheckEnabled:         output.HealthCheckEnabled,
		HealthCheckIntervalSeconds: output.HealthCheckIntervalSeconds,
		HealthCheckTimeoutSeconds:  output.HealthCheckTimeoutSeconds,
		HealthyThresholdCount:      output.HealthyThresholdCount,
		UnhealthyThresholdCount:    output.UnhealthyThresholdCount,
		Port:                       output.Port,
		Protocol:                   output.Protocol,
		TargetType:                 output.TargetType,
		VpcId:                      output.VpcId,
	}
}

func (svc *Service) reconcileLoadBalancer(loadbalancerName string) (*string, *string, error) {
	var (
		loadBalancerArn            *string
		loadBalancerDNS            *string
		awsLBConfig                = svc.awsLBConfig
		describeLoadBalancersInput = &elbv2.DescribeLoadBalancersInput{
			Names: []*string{aws.String(GetLoadBalancerNameForCluster(loadbalancerName))},
		}
	)
	describeLoadBalancersOutput, err := svc.ELB.DescribeLoadBalancers(describeLoadBalancersInput)
	if err != nil {
		if IsNotFound(err) {
			createLoadBalancerInput := &elbv2.CreateLoadBalancerInput{
				Name: aws.String(GetLoadBalancerNameForCluster(loadbalancerName)),
				Type: aws.String(elbv2.LoadBalancerTypeEnumNetwork),
			}
			for _, subnet := range awsLBConfig.Spec.SubnetIDs {
				createLoadBalancerInput.Subnets = append(createLoadBalancerInput.Subnets, &subnet)
			}
			createLoadBalancerOutput, err := svc.ELB.CreateLoadBalancer(createLoadBalancerInput)
			if err != nil {
				return nil, nil, err
			}
			loadBalancerArn = createLoadBalancerOutput.LoadBalancers[0].LoadBalancerArn
			loadBalancerDNS = createLoadBalancerOutput.LoadBalancers[0].DNSName

			return loadBalancerArn, loadBalancerDNS, nil
		}
		return nil, nil, err
	}

	loadBalancerArn = describeLoadBalancersOutput.LoadBalancers[0].LoadBalancerArn
	loadBalancerDNS = describeLoadBalancersOutput.LoadBalancers[0].DNSName
	svc.logger.V(6).Info("reconciled loadbalancer", "dns", loadBalancerDNS, "lodbalancerARN", loadBalancerArn)

	return loadBalancerArn, loadBalancerDNS, nil
}

// GetLoadBalancerNameForCluster returns the name of the load balancer for a given cluster.
func GetLoadBalancerNameForCluster(clusterName string) string {
	return fmt.Sprintf("%s-endpoint", clusterName)
}

func (svc *Service) reconcileListeners(loadBalancerArn *string, targetGroupArn *string, loadBalancer *infrav1.LoadBalancer) (*int64, error) {
	var (
		listenerPort         *int64
		desiredlistenerPort  *int64
		desiredProtocol      = aws.String(elbv2.ProtocolEnumTcp)
		describeListnerInput = &elbv2.DescribeListenersInput{
			LoadBalancerArn: loadBalancerArn,
		}
	)
	if len(loadBalancer.Spec.Ports) == 0 {
		return nil, errors.New("empty load balancer ports")
	}

	// TODO yastij: handle multi-port listner/target support
	lbPort := loadBalancer.Spec.Ports[0]
	desiredlistenerPort = aws.Int64(int64(lbPort.Port))

	describeListnerOutput, err := svc.ELB.DescribeListeners(describeListnerInput)
	if err != nil || len(describeListnerOutput.Listeners) == 0 {
		if IsNotFound(err) || len(describeListnerOutput.Listeners) == 0 {
			listenerInput := &elbv2.CreateListenerInput{
				LoadBalancerArn: loadBalancerArn,
				Port:            desiredlistenerPort,
				Protocol:        desiredProtocol,
			}

			listenerInput.DefaultActions = append(listenerInput.DefaultActions, &elbv2.Action{
				TargetGroupArn: targetGroupArn,
				Type:           aws.String(elbv2.ActionTypeEnumForward),
			})

			listenerOutput, err := svc.ELB.CreateListener(listenerInput)
			if err != nil {
				return nil, err
			}
			return listenerOutput.Listeners[0].Port, nil
		}
		return nil, err
	}

	modifyListenerInput := &elbv2.ModifyListenerInput{
		Port:        desiredlistenerPort,
		Protocol:    desiredProtocol,
		ListenerArn: describeListnerOutput.Listeners[0].ListenerArn,
	}
	modifyListenerInput.DefaultActions = append(modifyListenerInput.DefaultActions, &elbv2.Action{
		TargetGroupArn: targetGroupArn,
		Type:           aws.String(elbv2.ActionTypeEnumForward),
	})
	modifyListenerOutput, err := svc.ELB.ModifyListener(modifyListenerInput)
	if err != nil {
		return nil, err
	}
	listenerPort = modifyListenerOutput.Listeners[0].Port
	svc.logger.V(6).Info("reconciled listener", "listernerPort", listenerPort, "listenerARN", modifyListenerOutput.Listeners[0].ListenerArn)

	return listenerPort, nil
}

func (svc *Service) deleteLoadBalancer(loadBalancerName string) error {
	describeLoadBalancersInput := &elbv2.DescribeLoadBalancersInput{
		Names: []*string{aws.String(GetLoadBalancerNameForCluster(loadBalancerName))},
	}
	describeLoadBalancersOutput, err := svc.ELB.DescribeLoadBalancers(describeLoadBalancersInput)
	if err != nil {
		if IsNotFound(err) {
			return nil
		}
		return err
	}
	loadBalancerArn := describeLoadBalancersOutput.LoadBalancers[0].LoadBalancerArn
	deleteLoadBalancerInput := &elbv2.DeleteLoadBalancerInput{
		LoadBalancerArn: loadBalancerArn,
	}
	_, err = svc.ELB.DeleteLoadBalancer(deleteLoadBalancerInput)
	return err
}

func (svc *Service) deleteTargetGroup(loadBalancerName string) error {
	describeTargetGroupInput := &elbv2.DescribeTargetGroupsInput{
		Names: []*string{GetTargetGroupNameForCluster(loadBalancerName)},
	}
	describeTargetGroupOutput, err := svc.ELB.DescribeTargetGroups(describeTargetGroupInput)
	if err != nil {
		if IsNotFound(err) {
			return nil
		}
		return err
	}
	deleteTargetGroupInput := &elbv2.DeleteTargetGroupInput{
		TargetGroupArn: describeTargetGroupOutput.TargetGroups[0].TargetGroupArn,
	}
	_, err = svc.ELB.DeleteTargetGroup(deleteTargetGroupInput)
	return err
}

// Delete performs an ELB resource deletion
func (svc *Service) Delete(loadBalancer *infrav1.LoadBalancer) error {
	loadBalancerName := loadBalancer.Name
	if err := svc.deleteLoadBalancer(loadBalancerName); err != nil {
		return err
	}
	return svc.deleteTargetGroup(loadBalancerName)
}
