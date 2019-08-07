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
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/elbv2/elbv2iface"
)

var (
	sessionCache sync.Map
)

const apiEndpointPort = 6443

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

// Service TODO
type Service struct {
	ELB elbv2iface.ELBV2API
}

// New TODO
func New(region string) *Service {

	session, _ := sessionForRegion(region)

	return &Service{
		ELB: elbv2.New(session),
	}
}

func generateELBName(clusterName string) string {
	return fmt.Sprintf("%s-endpoint", clusterName)
}

// Reconcile reconciles loadbalancing resources in aws
func (svc *Service) Reconcile(vpcID string, controlPlaneIPs []string, clusterName string, subnets []string) (string, int, error) {

	targetGroupArn, err := svc.reconcileTargetGroup(clusterName, vpcID, controlPlaneIPs)
	if err != nil {
		return "", 0, err
	}

	loadBalancerArn, loadBalancerDNS, err := svc.reconcileLoadBalancer(clusterName, subnets)
	if err != nil {
		return "", 0, err
	}

	listenerPort, err := svc.reconcileListeners(loadBalancerArn, targetGroupArn)
	if err != nil {
		return "", 0, err
	}

	return *loadBalancerDNS, int(*listenerPort), nil
}

func (svc *Service) reconcileTargetGroup(clusterName string, vpcID string, controlPlaneIPs []string) (*string, error) {
	describeTargetGroupInput := &elbv2.DescribeTargetGroupsInput{}
	describeTargetGroupInput.Names = append(describeTargetGroupInput.Names, aws.String(string(clusterName+"-controlPlane")))

	describeTargetGroupOutput, err := svc.ELB.DescribeTargetGroups(describeTargetGroupInput)
	desiredTargetGroupInput := &elbv2.CreateTargetGroupInput{
		Name:                       aws.String(string(clusterName + "-controlPlane")),
		HealthCheckEnabled:         aws.Bool(true),
		HealthCheckIntervalSeconds: aws.Int64(int64(30 * time.Second.Seconds())),
		HealthCheckTimeoutSeconds:  aws.Int64(int64(10 * time.Second.Seconds())),
		HealthyThresholdCount:      aws.Int64(int64(3)),
		UnhealthyThresholdCount:    aws.Int64(int64(3)),
		Port:                       aws.Int64(int64(6443)),
		Protocol:                   aws.String(elbv2.ProtocolEnumTcp),
		TargetType:                 aws.String(string("ip")),
		VpcId:                      &vpcID,
	}

	var targetGroupArn *string
	if err != nil {
		if IsNotFound(err) {
			targetGroupOutput, err := svc.ELB.CreateTargetGroup(desiredTargetGroupInput)
			if err != nil {
				return nil, err
			}
			targetGroupArn = targetGroupOutput.TargetGroups[0].TargetGroupArn
		} else {
			return nil, err
		}
	}

	actualTargetGroupInput := convertTargetGroupOutputToInput(describeTargetGroupOutput.TargetGroups[0])
	if !reflect.DeepEqual(actualTargetGroupInput, desiredTargetGroupInput) {
		modifyTargetGroup := &elbv2.ModifyTargetGroupInput{
			HealthCheckEnabled:         desiredTargetGroupInput.HealthCheckEnabled,
			HealthCheckIntervalSeconds: desiredTargetGroupInput.HealthCheckIntervalSeconds,
			HealthCheckTimeoutSeconds:  desiredTargetGroupInput.HealthCheckTimeoutSeconds,
			HealthyThresholdCount:      desiredTargetGroupInput.HealthyThresholdCount,
			UnhealthyThresholdCount:    desiredTargetGroupInput.UnhealthyThresholdCount,
		}
		_, err := svc.ELB.ModifyTargetGroup(modifyTargetGroup)
		if err != nil {
			return nil, err
		}
	}
	targetGroupArn = describeTargetGroupOutput.TargetGroups[0].TargetGroupArn

	if len(controlPlaneIPs) != 0 {
		targetDescription := []*elbv2.TargetDescription{}
		for _, controlPlaneIP := range controlPlaneIPs {
			targetDescription = append(targetDescription, &elbv2.TargetDescription{
				AvailabilityZone: aws.String(string("all")),
				Id:               aws.String(controlPlaneIP),
				Port:             aws.Int64(int64(6443)),
			})
		}

		registerTargetInput := &elbv2.RegisterTargetsInput{
			TargetGroupArn: targetGroupArn,
			Targets:        targetDescription,
		}
		_, err = svc.ELB.RegisterTargets(registerTargetInput)
		if err != nil {
			return nil, err
		}
	}
	return targetGroupArn, nil
}

func convertTargetGroupOutputToInput(output *elbv2.TargetGroup) *elbv2.CreateTargetGroupInput {
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

func (svc *Service) reconcileLoadBalancer(clusterName string, subnets []string) (*string, *string, error) {
	var loadBalancerArn *string
	var loadBalancerDNS *string
	describeLoadBalancersInput := &elbv2.DescribeLoadBalancersInput{
		Names: []*string{aws.String(generateELBName(clusterName))},
	}
	describeLoadBalancersOutput, err := svc.ELB.DescribeLoadBalancers(describeLoadBalancersInput)
	if err != nil {
		if IsNotFound(err) {
			createLoadBalancerInput := &elbv2.CreateLoadBalancerInput{
				Name: aws.String(generateELBName(clusterName)),
				Type: aws.String(elbv2.LoadBalancerTypeEnumNetwork),
			}
			for _, subnet := range subnets {
				createLoadBalancerInput.Subnets = append(createLoadBalancerInput.Subnets, &subnet)
			}
			createLoadBalancerOutput, err := svc.ELB.CreateLoadBalancer(createLoadBalancerInput)
			if err != nil {
				return nil, nil, err
			}
			loadBalancerArn = createLoadBalancerOutput.LoadBalancers[0].LoadBalancerArn
			loadBalancerDNS = createLoadBalancerOutput.LoadBalancers[0].DNSName
		} else {
			return nil, nil, err
		}
	}
	loadBalancerArn = describeLoadBalancersOutput.LoadBalancers[0].LoadBalancerArn
	loadBalancerDNS = describeLoadBalancersOutput.LoadBalancers[0].DNSName

	return loadBalancerArn, loadBalancerDNS, nil
}

func (svc *Service) reconcileListeners(loadBalancerArn *string, targetGroupArn *string) (*int64, error) {
	var listenerPort *int64
	describeListnerInput := &elbv2.DescribeListenersInput{
		LoadBalancerArn: loadBalancerArn,
	}
	describeListnerOutput, err := svc.ELB.DescribeListeners(describeListnerInput)
	if err != nil {
		if IsNotFound(err) || len(describeListnerOutput.Listeners) == 0 {
			listenerInput := &elbv2.CreateListenerInput{
				LoadBalancerArn: loadBalancerArn,
				Port:            aws.Int64(int64(6443)),
				Protocol:        aws.String(elbv2.ProtocolEnumTcp),
			}

			listenerInput.DefaultActions = append(listenerInput.DefaultActions, &elbv2.Action{
				TargetGroupArn: targetGroupArn,
				Type:           aws.String(elbv2.ActionTypeEnumForward),
			})

			listenerOutput, err := svc.ELB.CreateListener(listenerInput)
			if err != nil {
				return nil, err
			}
			listenerPort = listenerOutput.Listeners[0].Port
		}
	}
	modifyListenerInput := &elbv2.ModifyListenerInput{
		Port:        aws.Int64(int64(6443)),
		Protocol:    aws.String(elbv2.ProtocolEnumTcp),
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
	return listenerPort, nil
}

func (svc *Service) deleteLoadBalancer(clusterName string) error {

	describeLoadBalancersInput := &elbv2.DescribeLoadBalancersInput{
		Names: []*string{aws.String(generateELBName(clusterName))},
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

func (svc *Service) deleteTargetGroup(clusterName string) error {
	describeTargetGroupInput := &elbv2.DescribeTargetGroupsInput{}
	describeTargetGroupInput.Names = append(describeTargetGroupInput.Names, aws.String(string(clusterName+"-controlPlane")))

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
func (svc *Service) Delete(clusterName string) error {
	if err := svc.deleteLoadBalancer(clusterName); err != nil {
		return err
	}

	if err := svc.deleteTargetGroup(clusterName); err != nil {
		return err
	}

	return nil
}
