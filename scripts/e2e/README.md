Test cluster-api-provider-vsphere

***Integration with Prow***   
apply hack/secret.yml to Prow cluster/local cluster   
apply hack/job.yml at Prow cluster/local cluster   
note: the actual Prow job definition file will be at k8s.io/test-infra   

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

 
***Launch CI from travis-ci***  
```
docker run \
  --rm \
  -v $HOME/.ssh:/root/ssh \
  -e GOVC_URL=$GOVC_URL \
  -e GOVC_USERNAME=$GOVC_USERNAME \
  -e GOVC_PASSWORD=$GOVC_PASSWORD \
  -e JUMPHOST=$JUMPHOST \
  -e GOVC_INSECURE="true" \
  -e VSPHERE_MACHINE_CONTROLLER_REGISTRY=$VSPHERE_MACHINE_CONTROLLER_REGISTRY \
  -ti luoh/cluster-api-provider-vsphere-travis-ci:latest
```
note: set `$VSPHER_MACHINE_CONTROLLER_REGISTRY` if you want to test your local build controller
   
   
***Architecture***  
```

                                             +-----------------------------------+
      +----------------------+               |          VMC Infra                |
      |   travis-ci env      |               |-----------------------------------|
      |----------------------|               |+----+ +--------------------------+|
      |                      |               ||    | |  bootstrap cluster       ||
      |                      |               ||    | |                          ||
      | cluster-api-vsphere- |               ||JUMP| |  cluster-api-vsphere-ci  ||
      | travis-ci            |  SSH + HTTP   ||HOST| |  (a k8s job)             ||
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
      
***Containers***  
the vsphere-machine-controller containers for CI purpose are hosted at   
`gcr.io/cnx-cluster-api/vsphere-cluster-api-provider`   
the cluster-api-provider-vsphere-travis-ci hosted at   
`luoh/cluster-api-provider-vsphere-travis-ci`   
the cluster-api-provider-vsphere-ci hosted at   
`gcr/cnx-cluster-api/cluster-api-provider-vsphere-ci`   
