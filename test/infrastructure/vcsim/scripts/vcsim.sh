#!/bin/bash

# Copyright 2024 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

VCENTERSIMULATOR_NAME=$1
CLUSTER_NAME=$2

# Validate params
if [ -z "$VCENTERSIMULATOR_NAME" ]
then
      echo "ERROR: VCenterSimulator name missing. usage: vcsim-prepare <vcentersimulator-name> <cluster>"
      exit 1
fi

if [ -z "$CLUSTER_NAME" ]
then
      echo "ERROR: Workload cluster name missing. usage: vcsim-prepare <vcentersimulator-name> <cluster>"
      exit 1
fi

# Check VCenterSimulator exists or create it
if eval "kubectl get VCenterSimulator $VCENTERSIMULATOR_NAME &> /dev/null"; then
  echo "using existing VCenterSimulator $VCENTERSIMULATOR_NAME"
else
  kubectl apply -f - &> /dev/null <<EOF
apiVersion: vcsim.infrastructure.cluster.x-k8s.io/v1alpha1
kind: VCenterSimulator
metadata:
  name: $VCENTERSIMULATOR_NAME
EOF
  echo "created VCenterSimulator $VCENTERSIMULATOR_NAME"
fi

# Check FakeAPIServerEndpoint exists or create it
if eval "kubectl get controlplaneendpoint $CLUSTER_NAME &> /dev/null"; then
  echo "using existing ControlPlaneEndpoint $CLUSTER_NAME"
else
  kubectl apply -f - &> /dev/null <<EOF
apiVersion: vcsim.infrastructure.cluster.x-k8s.io/v1alpha1
kind: ControlPlaneEndpoint
metadata:
  name: $CLUSTER_NAME
EOF
  echo "created ControlPlaneEndpoint $CLUSTER_NAME"
  sleep 3
fi

# Check EnvVar exists or create it
CURRENT_NAMESPACE=$(kubectl config view --minify --output 'jsonpath={..namespace}')
CLUSTER_NAMESPACE=${CURRENT_NAMESPACE:-default}
if eval "kubectl get envvar $CLUSTER_NAME &> /dev/null"; then
  echo "using existing EnvVar $CLUSTER_NAME"
else
  kubectl apply -f - <<EOF
apiVersion: vcsim.infrastructure.cluster.x-k8s.io/v1alpha1
kind: EnvVar
metadata:
  name: $CLUSTER_NAME
spec:
  vCenterSimulator:
    name: $VCENTERSIMULATOR_NAME
  controlPlaneEndpoint:
    name: $CLUSTER_NAME
  cluster:
    name: $CLUSTER_NAME
    namespace: $CLUSTER_NAMESPACE
EOF
  echo "created EnvVar $CLUSTER_NAME"
fi

i=0
maxRetry=10
while true; do
    status=$(kubectl get envvar "$CLUSTER_NAME" -o json | jq ".status")
    if [ -n "$status" ] && [ "$status" != "null" ]; then
      break
    fi
    sleep 1
    if [ "$i" -ge "$maxRetry" ]
    then
      echo "ERROR: EnvVar $CLUSTER_NAME is not being reconciled; check vcsim controller logs"
      exit 1
    fi
    (( i++ ))
done

# Get all the variables from EnvVar

kubectl get envvar "$CLUSTER_NAME" -o json | jq ".status.variables | to_entries | map(\"export \\(.key)=\\\"\\(.value|tostring)\\\"\") | .[]" -r > vcsim.env

echo "done!"
echo "GOVC_URL=$(kubectl get envvar "$CLUSTER_NAME" -o json | jq -r ".status.variables.GOVC_URL")"
echo
echo "source vcsim.env"
