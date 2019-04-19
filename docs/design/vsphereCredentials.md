## Passing vsphere credentials
For the cluster-api vsphere provider to work, the users need to provide the vsphere credentials to access the infrastructure. There are 2 ways how the users can provide these credentials.

* Using kubernetes `secrets`
   * Create a secret that contains 2 keys namely `username` and `password` in the same namespace as the desired `Cluster` object.
      ```
      apiVersion: v1
      kind: Secret
      metadata:
        name: my-vc-credentials
      type: Opaque
      data:
        # base64 encoded fields
        username: YWRtaW5pc3RyYXRvckB2c3BoZXJlLmxvY2Fs
        password: c2FtcGxl
      ```

   * Set the `vsphereCredentialSecret` property in the `ProviderSpec` part of the `Cluster` definition
      ```
      apiVersion: "cluster.k8s.io/v1alpha1"
      kind: Cluster
      metadata:
        name: sample-cluster
      spec:
          ...
          providerSpec:
            value:
              ...
              # Credentials provided via secrets
              vsphereCredentialSecret: "my-vc-credentials"
      ```

* Using plain text credential in the `ProviderSpec` part of the `Cluster` definition
    ```
    apiVersion: "cluster.k8s.io/v1alpha1"
    kind: Cluster
    metadata:
      name: sample-cluster
    spec:
        ...
        providerSpec:
          value:
            ...
            # Credentials provided as plain text
            vsphereUser: "administrator@vsphere.local"
            vspherePassword: "sample"
    ```

__Note:__ If `vsphereCredentialSecret` field is set to a non empty string then the controller will ignore the `vsphereUser` and `vspherePassword` fields even if they are set.