# Test cluster-api-provider-vsphere

## Prow

note: the actual Prow job definition file is at k8s.io/test-infra  
test-infra/config/jobs/kubernetes-sigs/cluster-api-provider-vsphere  

```ascii
            +-----------------------------------------------------+
            |                                                     |
            |                                                     |
            |        container running on Prow cluster:           |
            |                                                     |
            |        create bootstrap cluster (on VMC)            |
            |        transfer secret from Prow to bootstrap       |
            |        launch a ci job at bootstrap                 |
            |        monitor job status                           |
            |                                                     |
            |                                                     |
            |                             +---------------------+ |
            |                             |  secret             | |
            |                             +---------------------+ |
            +-----------------------------------------------------+

           +-------------------------------------------------------+
           |        +--------------------------------------------+ |
           |        |  secret: target VM SSH, bootstrap cluster  | |
           |        |  kubeconfig, vsphere info                  | |
           |        |                                            | |
           |        +--------------------------------------------+ |
           |                                                       |
           |                             +-----------------------+ |
           |                             |                       | |
           |                             |     CI job:           | |
           |                             | create target cluster | |
           |                             | on VMC                | |
           |                             +-----------------------+ |
           |                                                       |
           |        BOOTSTRAP CLUSTER (on VMC)                     |
           |                                                       |
           +-------------------------------------------------------+
```

## Architecture

```ascii

                                             +-----------------------------------+
      +----------------------+               |          VMC Infra                |
      |   Prow/Local cluster |               |-----------------------------------|
      |----------------------|               |+----+ +--------------------------+|
      |                      |               ||    | |  bootstrap cluster       ||
      |                      |               ||    | |                          ||
      | cluster-api-vsphere- |               ||JUMP| |  cluster-api-vsphere-ci  ||
      | -ci                  |  SSH + HTTP   ||HOST| |  (a k8s job)             ||
      |                      | +-----------> ||    | |                          ||
      |                      | <-----------+ ||    | |                          ||
      |                      |               ||    | +--------------------------+|
      |                      |               ||    |                             |
      |                      |               ||    | +--------------------------+|
      |                      |               ||    | |  target cluster          ||
      |                      |               ||    | |                          ||
      |                      |               ||    | |                          ||
      |                      |               |+----+ +--------------------------+|
      +----------------------+               +-----------------------------------+
```

## Containers

The CAPV manager images are hosted at [`gcr.io/cluster-api-provider-vsphere`](gcr.io/cluster-api-provider-vsphere).

## Test CI locally

### Prerequisites

1. A local cluster
2. Apply the secret (based on secret.template) to local cluster

### Steps

1. Build the CI container

    ```shell
    cd ./scripts/e2e/hack && make build
    ```

2. Create the job

    ```shell
    kubectl create -f job.yml
    ```
