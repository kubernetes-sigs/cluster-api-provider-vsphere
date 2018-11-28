# Test

## Integration tests

These tests were written with ginkgo and gomega.  The intent of these test is that they will run after a deployment.  The tests will verify the deployment is correct.  The tests have been divided up into several categories.  Running a specific category of tests can be done using ginkgo's --focus arguments and a regular expression.  In addition, the tests were written not to care about a specific cluster.  They can be run against a bootstrap cluster (management cluster) or a target cluster by passing in the path of the cluster's kubeconfig file in env var _TEST_KUBECONFIG_.

#### Categories of tests

| Category | Description |
| --- | --- |
| environment | verifies the environment.  For example, verify the bootstrap cluster is clear of cluster api. |
| create | verifies the deployment of a cluster |
| update | verifies the update of a cluster |
| delete | verifies the deletion of a cluster |

#### Input environment variables

| Variable | Description |
| --- | --- |
| TEST_KUBECONFIG | path of the kubeconfig file for the cluster to target tests |
| TEST_CLUSTERNAME | name of cluster to verify.  This variable is needed for the _deploy_ category of tests. |
| TEST_CLUSTERNAMESPACE | namespace of the cluster to verify.  This variable is needed for the _deploy_ category of tests. |

#### Example execution

```
// Create a single master cluster
$> clusterctl create cluster ...

// Test the target cluster.  Assumes clusterctl pulled the target cluster's kubeconfig to the current path.
$> TEST_KUBECONFIG=$pwd/kubeconfig TEST_CLUSTERNAME=test1 TEST_CLUSTERNAMESPACE=default ginkgo --focus="create single master"

```

Note, the name `create single master` in the focus argument is a partial string for the test name in the file create_singlemachine_test.go.

```
// Create a bootstrap cluster
$> minikube start ...

// Test the bootstrap cluster is clear of cluster api
$> TEST_KUBECONFIG=$pwd/kubeconfig ginkgo --focus="environment"

```