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

package ssh_test

import (
	"bytes"
	"testing"

	"k8s.io/klog/klogr"

	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/apis/vsphere/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/context"
	"sigs.k8s.io/cluster-api-provider-vsphere/pkg/cloud/vsphere/services/ssh"
)

func TestReconcile(t *testing.T) {
	testCases := []struct {
		name             string
		inputAuthKeys    []string
		inputKeyPair     *v1alpha1.KeyPair
		expectKeyPairGen bool
		expectedError    error
	}{
		{
			name:             "should generate keypair when inputKeyPair==nil",
			expectKeyPairGen: true,
			expectedError:    nil,
			inputAuthKeys:    authKeys,
		},
		{
			name:             "should generate keypair when inputKeyPair has no cert",
			expectKeyPairGen: true,
			expectedError:    nil,
			inputAuthKeys:    authKeys,
			inputKeyPair:     &v1alpha1.KeyPair{Key: []byte(sshPrvKey)},
		},
		{
			name:             "should generate keypair when inputKeyPair has no key",
			expectKeyPairGen: true,
			expectedError:    nil,
			inputAuthKeys:    authKeys,
			inputKeyPair:     &v1alpha1.KeyPair{Cert: []byte(sshPubKey)},
		},
		{
			name:             "should generate keypair when inputKeyPair has no cert and nokey",
			expectKeyPairGen: true,
			expectedError:    nil,
			inputAuthKeys:    authKeys,
			inputKeyPair:     &v1alpha1.KeyPair{},
		},
		{
			name:             "should not generate keypair when inputKeyPair has cert and key",
			expectKeyPairGen: false,
			expectedError:    nil,
			inputAuthKeys:    authKeys,
			inputKeyPair:     &v1alpha1.KeyPair{Cert: []byte(sshPubKey), Key: []byte(sshPrvKey)},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := &context.ClusterContext{
				Logger: klogr.New().WithName("default-logger"),
				ClusterConfig: &v1alpha1.VsphereClusterProviderSpec{
					SSHAuthorizedKeys: tc.inputAuthKeys,
				},
			}
			if tc.inputKeyPair != nil {
				ctx.ClusterConfig.SSHKeyPair = *tc.inputKeyPair
			}
			err := ssh.Reconcile(ctx)
			if err != tc.expectedError {
				t.Fatalf("error does not equal expected: exp=%v act=%v", tc.expectedError, err)
				return
			}
			if tc.inputKeyPair != nil && len(tc.inputKeyPair.Cert) > 0 && len(tc.inputKeyPair.Key) > 0 {
				if !bytes.Equal(ctx.ClusterConfig.SSHKeyPair.Cert, tc.inputKeyPair.Cert) {
					t.Fatal("generated public key does not match expected public key")
					return
				}
				if !bytes.Equal(ctx.ClusterConfig.SSHKeyPair.Key, tc.inputKeyPair.Key) {
					t.Fatal("generated private key does not match expected private key")
					return
				}
			}
			if tc.expectKeyPairGen && !ctx.ClusterConfig.SSHKeyPair.HasCertAndKey() {
				t.Fatal("failed to generate SSH key pair")
				return
			}
			actAuthKeys := ctx.ClusterConfig.GetSSHAuthorizedKeys()
			expAuthKeyCount := len(tc.inputAuthKeys) + 1
			if actAuthKeyCount := len(actAuthKeys); actAuthKeyCount != expAuthKeyCount {
				t.Fatalf("invalid auth keys count: exp=%v act=%v", expAuthKeyCount, actAuthKeyCount)
				return
			}
			if len(tc.inputAuthKeys) > 0 {
				for i := range tc.inputAuthKeys {
					if tc.inputAuthKeys[i] != actAuthKeys[i] {
						t.Fatal("expected auth key does not match actual auth key")
						return
					}
				}
				lastActAuthKey := actAuthKeys[len(actAuthKeys)-1]
				if lastActAuthKey != string(ctx.ClusterConfig.SSHKeyPair.Cert) {
					t.Fatal("expected auth key does not match actual auth key from key pair")
					return
				}
			}
		})
	}
}

var authKeys = []string{
	"ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDFFH2PAbjqc//QibjQCcZudKQb19R3SvZ/C8kjfhsxUQvacEguJh3xk/wsfWrLsQRi6ZcPj2ISNLF3vilFYbIW2nLPrhMxlmUVav8jAbJrN5Q4vhXQ6zIvJES5XdHo6Helq9W8vevIqDFE6nIu74OWq8nasIgFuwFLEt/NWZz0yYr2tbxh2U/rFhdyWwG6vDFkEuMR9GBsirbF1/v75aLvaG8NPFk+dab1uSyuGUCDlPNdl+ySxKZvcdegDC3dHIXzZE6i2OlL+Q1cfVp6ihdHq7XZaYYkXsdxAC/Uecnn6cD4IoyxZPpod2WohhOV9yjzwueTqHhY5mu1gJjsGAfH",
	"ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDO8rLzD7p8R6euzAEI+XATEaW6Hbpp7jJ2dhHNQ2cd1+0lhdP+Htkaremud5tPyFnbZof3TsDLCFvvVIppHeNnuUxWigF/tcRy51yK1Db0+7pcZQbM0qBe3ZfDMbuet0tp4Db1mAFMkMizcQBnL3BhPC5taDAOFUVrAJXkw5xtyg9DoZvnvLLQNGJ0AOSXVmxJzXX//1dbSPybQ2LpB+fyyfAe+861+zg6qe8lc2QtLdFiZxvmKQAeZ27SZvrtZkEEVHDz8b7DnnQFykMRRFFGNJZnXjsPpU57o3VnNEsvshC4cpuJwebfYcw88WGpVHOjOUfm2VMrht8Kpqb2gFeZ",
}

const (
	sshPubKey = `ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDqDLEV7znSFMDb3RnRVOeIyzp+SH8BelvP5s5Ckx91iEG7yC8y4lTub5y8jZ92NDBCwlZ9pJ6UM8LTc6Ho3bPTu2d/nuKyIKou7mHJtgAIIOcQ0nUlyBKY0PF/hvJxXUlyhTUCigCOR2BLw+J6jnKHwb8/ZQabHAEGAU8x5Q9oYMq+oV35nM9nQ4NTHGP2d5P2w/o37XfLkRzkSgMS/sCbOA6mbeNWRMzqCZYDMv0dbYqD9ZYSnOxKa7/4M+YaLWLD4RNyAscrDK10/q5v260Hlh1RK86Dc4KFNuxe6v4FyZ4UWgbto44A96qd9zIhNOrGji3r2nwQaicRbS74u7ff`
	sshPrvKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEpQIBAAKCAQEA6gyxFe850hTA290Z0VTniMs6fkh/AXpbz+bOQpMfdYhBu8gv
MuJU7m+cvI2fdjQwQsJWfaSelDPC03Oh6N2z07tnf57isiCqLu5hybYACCDnENJ1
JcgSmNDxf4bycV1JcoU1AooAjkdgS8Pieo5yh8G/P2UGmxwBBgFPMeUPaGDKvqFd
+ZzPZ0ODUxxj9neT9sP6N+13y5Ec5EoDEv7AmzgOpm3jVkTM6gmWAzL9HW2Kg/WW
EpzsSmu/+DPmGi1iw+ETcgLHKwytdP6ub9utB5YdUSvOg3OChTbsXur+BcmeFFoG
7aOOAPeqnfcyITTqxo4t69p8EGonEW0u+Lu33wIDAQABAoIBADYNDkxxfdntXwin
jBHS2NG3lV+aoHIX7uIZfGLVlTtQZ1XVikjnChQyhHDrB/uFW+ve85h6jwDM315z
4t1jbeck7WcEq3fVoVfLR5wMwv8dkh9JazJ5fQn7nvoDkTPrBk5DQxW+BxjUlQGK
UGBbS0nczaz3SMpDcl0Pqllse91vo+QQJELJ53+EboQDnbaa4wc3F9J1y0lshA4Q
+ynaI34nirJdFUjG6K6IGRvd6jWn39BkSC7GEXjCcrYj3FaF9d4njQQfxlXw2tfT
y2DMIrUJg8Ha06zOgd07IHIOzxzeTXT2UUImq1hexA7n2EmFMbuevItbvkWQ8VLA
VQohyKkCgYEA7N/fqiaE/MH/rB+/zeqz3E/MEUyMqPueKJHQ9gIoPyJtIrTQd9yS
/gWvH95/gcWUOM5z/1hn7HkO8cl4Pe0Rkre5u2MkF2+J75hCDKWg9lZestnL8I+b
++WIMJrHjDweRNexLA9hYRy2ky154ZMP00m1FrpJ0sUxhPNmvMWBN8sCgYEA/PJt
dot1Jt9RCIhSFjyab3oB/X61mq2mX+exAAcOZvHu6wgzDQcLlQ3ZI8md0CjO9af5
GLJsEYwMa4cRRQ0VpjL/wuUpk2fU7rvCg8YTnaftO45rQiSWEpQHVjVxp2l4S5l0
vONM210FfqgoFF4VNG5yJYS+07oXe5u44UlJtb0CgYEA3i6ffNnko7DUQH8HSf57
9opiv1cuGNLq5uLfPeGIHrAL7iHr6IHc3qg2O45Xy0GoZiBAbaJe2FA01FZFktBr
S1NJw5qan+DfYP1P9szkzir1aI0h3eLWTNBfjjegNMmvGqO2a72BebWVCzf8urlW
frkEQu05kZmleS9VjnszWUECgYEApxcNsC1XaiJCyTwj3YSTD+isv+Of21myec/3
YGlI3kAa7y8vaf+pawEG21kn4oXSkPww1Fuof77fxXgntFF8Z5lw0jHHURRZ2Io3
aAzEkHSJhboCqGK6r/MRFaWgOlK1oFryfoQ4FQBRzOUP9MRhhY0f4iDaXcqkEIdB
jbB3/JECgYEA4fU+YX9Q94IwIX3rrxns4omV/E9XyTZBMXIOpM+lUvnrl7ig67MI
3RNj0o9vy3kUnxErjO+vzAyGQ8oPC3fLyNGEHicw1rA5QlGzanBKvldUvlU1zh40
/HddEUEbEcZhKVTtOAYrsEu4HSzxDssTDPJ41cH3+tFQIUg/036Qtwk=
-----END RSA PRIVATE KEY-----`
)
