/*
Copyright 2025 The Kubernetes Authors.

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

package kubernetes

import (
	"context"
)

// workerPodHandler implement handling for the Pod hosting a minimal Kubernetes worker.
type workerPodHandler struct {
	// TODO: implement using kubemark or virtual kubelet.
	//  kubermark seems the best fit
	//  virtual kubelet with the mock provider seems a possible alternative, but I don't know if the mock providers has limitations that might limit usage.
	//  virtual kubelet with other providers seems overkill in this phase
}

func (p *workerPodHandler) LookupAndGenerateRBAC(ctx context.Context) error {

	return nil
}

func (p *workerPodHandler) Generate(ctx context.Context, kubernetesVersion string) error {

	return nil
}

func (p *workerPodHandler) Delete(ctx context.Context, podName string) error {

	return nil
}
