/*
Copyright 2019 The Kubernetes Authors.

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

package kind

import (
	"context"
	"fmt"
	"strings"

	"sigs.k8s.io/cluster-api/test/framework/exec"
)

// TeardownIfExists removes a Kind cluster if it exists.
func TeardownIfExists(ctx context.Context, clusterName string) error {
	listCmd := exec.NewCommand(
		exec.WithCommand("kind"),
		exec.WithArgs("get", "clusters"),
	)
	stdout, stderr, err := listCmd.Run(ctx)
	if err != nil {
		fmt.Println(string(stdout))
		fmt.Println(string(stderr))
		return err
	}
	if strings.Contains(string(stdout), clusterName) {
		deleteCmd := exec.NewCommand(
			exec.WithCommand("kind"),
			exec.WithArgs("delete", "cluster", "--name", clusterName),
		)
		stdout, stderr, err := deleteCmd.Run(ctx)
		if err != nil {
			fmt.Println(string(stdout))
			fmt.Println(string(stderr))
			return err
		}
	}
	return nil
}
