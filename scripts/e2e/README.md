Test cluster-api-provider-vsphere

**Prow**   
note: the actual Prow job definition file is at [k8s.io/test-infra](test-infra/config/jobs/kubernetes-sigs/cluster-api-provider-vsphere)    

```
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
 
   
**Architecture**    
```

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
      
**Containers**    
the vsphere-machine-controller containers for CI purpose are hosted at   
`gcr.io/cnx-cluster-api/vsphere-cluster-api-provider`   
the cluster-api-provider-vsphere-ci hosted at   
`gcr/cnx-cluster-api/cluster-api-provider-vsphere-ci` 


**Test CI locally (non-Prow)**   
****Prerequisite**** 
1) A local cluster (prefer minikube)   
2) Apply the secret (based on secret.template) to local cluster   
 
**Steps with minikube**   
`cd ./scripts/e2e/hack && make build`   
this will build ci container that contains cluster-api-provider-vsphere code from your working directory.    

this is only necessary when we want minikube to pull image from local docker images   
`eval $(minikube docker-env)`   

`kubectl create -f job.yml`   
and monitor the job status   
