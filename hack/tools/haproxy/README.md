# haproxy

HAProxy and its [dataplane API](https://www.haproxy.com/documentation/dataplaneapi/latest) provide a remotely-configurable, generic load balancer for CAPV.

## Overview

This document provides:

* [A quick and easy demonstration of HAProxy as a load-balancer using containers](#quickstart)
* [Instructions for building a light-weight, HAProxy load-balancer OVA](#ova)

## Quickstart

Illustrating the utility of haproxy as a load-balancer is best accomplished using a container:

1. Build the image:

    ```shell
    docker build -t haproxy .
    ```

2. Start the haproxy image in detached mode and map its secure, dataplane API port (`5556`) and the port used by the load balancer (`8085`) to the local host:

    ```shell
    docker run -it --name haproxy -d --rm -p 5556:5556 -p 8085:8085 haproxy
    ```

3. Create a [frontend configuration](https://www.haproxy.com/documentation/dataplaneapi/latest/#tag/Frontend):

    ```shell
    $ curl -X POST \
      --cacert example/ca.crt \
      --cert example/client.crt --key example/client.key \
      --user client:cert \
      -H "Content-Type: application/json" \
      -d '{"name": "lb-frontend", "mode": "tcp"}' \
      "https://localhost:5556/v1/services/haproxy/configuration/frontends?version=1"
    {"mode":"tcp","name":"lb-frontend"}
    ```

4. [Bind](https://www.haproxy.com/documentation/dataplaneapi/latest/#tag/Bind) the frontend configuration to `*:8085`:

    ```shell
    $ curl -X POST \
      --cacert example/ca.crt \
      --cert example/client.crt --key example/client.key \
      --user client:cert \
      -H "Content-Type: application/json" \
      -d '{"name": "lb-frontend", "address": "*", "port": 8085}' \
      "https://localhost:5556/v1/services/haproxy/configuration/binds?frontend=lb-frontend&version=2"
      {"address":"*","name":"lb-frontend","port":8085}
    ```

5. At this point it is possible to curl the load balancer, even if there is no one on the backend answering the query:

    ```shell
    $ curl http://localhost:8085
    curl: (52) Empty reply from server
    ```

6. Create a [backend configuration](https://www.haproxy.com/documentation/dataplaneapi/latest/#tag/Backend) and bind it to the frontend configuration:

    ```shell
    $ curl -X POST \
      --cacert example/ca.crt \
      --cert example/client.crt --key example/client.key \
      --user client:cert \
      -H "Content-Type: application/json" \
      -d '{"name": "lb-backend", "mode":"tcp", "balance": {"algorithm":"roundrobin"}, "adv_check": "tcp-check"}' \
      "https://localhost:5556/v1/services/haproxy/configuration/backends?version=3"
      {"adv_check":"tcp-check","balance":{"algorithm":"roundrobin","arguments":null},"mode":"tcp","name":"lb-backend"}
    ```

7. Update the frontend to use the backend:

    ```shell
    $ curl -X PUT \
      --cacert example/ca.crt \
      --cert example/client.crt --key example/client.key \
      --user client:cert \
      -H "Content-Type: application/json" \
      -d '{"name": "lb-frontend", "mode": "tcp", "default_backend": "lb-backend"}' \
      "https://localhost:5556/v1/services/haproxy/configuration/frontends/lb-frontend?version=4"
      {"default_backend":"lb-backend","mode":"tcp","name":"lb-frontend"}
    ```

8. Run two simple web servers in detached mode named `http1` and `http2`:

    ```shell
    docker run --rm -d -p 8086:80 --name "http1" nginxdemos/hello:plain-text && \
    docker run --rm -d -p 8087:80 --name "http2" nginxdemos/hello:plain-text
    ```

9. Add the first web [server](https://www.haproxy.com/documentation/dataplaneapi/latest/#tag/Server) to the backend configuration:

    ```shell
    $ curl -X POST \
      --cacert example/ca.crt \
      --cert example/client.crt --key example/client.key \
      --user client:cert \
      -H "Content-Type: application/json" \
      -d '{"name": "lb-backend-server-1", "address": "'"$(docker inspect http1 -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}')"'", "port": 80, "check": "enabled", "maxconn": 30, "weight": 100}' \
      "https://localhost:5556/v1/services/haproxy/configuration/servers?backend=lb-backend&version=5"
      {"address":"172.17.0.2","check":"enabled","maxconn":30,"name":"lb-backend-server-1","port":80,"weight":100}
    ```

10. With the first web server attached to the load balancer's backend configuration, it should now be possible to query the load balancer and get more than an empty reply:

    ```shell
    $ curl http://localhost:8085
    Server address: 172.17.0.2:80
    Server name: 456dbfd57701
    Date: 21/Dec/2019:22:22:22 +0000
    URI: /
    Request ID: 7bcabcecb553bcee5ed7efb4b8725f96
    ```

    Sure enough, the server address `172.17.0.2` is the same as the reported IP address of `lb-backend-server-1` above!

11. Add the second web server to the backend configuration:

    ```shell
    $ curl -X POST \
      --cacert example/ca.crt \
      --cert example/client.crt --key example/client.key \
      --user client:cert \
      -H "Content-Type: application/json" \
      -d '{"name": "lb-backend-server-2", "address": "'"$(docker inspect http2 -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}')"'", "port": 80, "check": "enabled", "maxconn": 30, "weight": 100}' \
      "https://localhost:5556/v1/services/haproxy/configuration/servers?backend=lb-backend&version=6"
      {"address":"172.17.0.3","check":"enabled","maxconn":30,"name":"lb-backend-server-2","port":80,"weight":100}
    ```

12. Now that both web servers are connected to the load-balancer, use `curl` to query the load-balanced endpoint a few times to validate that both backend servers are being used:

    ```shell
    $ for i in 1 2 3 4; do curl http://localhost:8085 && echo; done
    Server address: 172.17.0.2:80
    Server name: 456dbfd57701
    Date: 21/Dec/2019:22:26:51 +0000
    URI: /
    Request ID: 77918aee58dd1eb7ba068b081d843a7c

    Server address: 172.17.0.3:80
    Server name: 877362812ed9
    Date: 21/Dec/2019:22:26:51 +0000
    URI: /
    Request ID: 097ccb892b565193f334fb544239fca6

    Server address: 172.17.0.2:80
    Server name: 456dbfd57701
    Date: 21/Dec/2019:22:26:51 +0000
    URI: /
    Request ID: 61022aa3a8a37cdf37541ec1c24b383e

    Server address: 172.17.0.3:80
    Server name: 877362812ed9
    Date: 21/Dec/2019:22:26:51 +0000
    URI: /
    Request ID: 2b2b9a0ef2e4eba53f6c5c118c10e1d8
    ```

    It's alive!

13. Stop haproxy and kill the web servers:

    ```shell
    docker kill haproxy http1 http2
    ```

## OVA

In production the haproxy loadbalancer is deployed as an OVA.

### OVA Requirements

Building the OVA requires:

* VMware Fusion or Workstation
* Packer 1.4.1
* Ansible 2.8+

### Building the OVA

To build the OVA please run the following the command:

```shell
make build
```

The above command build the OVA with Packer in _headless_ mode, meaning that VMware Fusion/Workstation will not display the virtual machine (VM) as it is being built. If the build process fails or times out, please use the following command to build the OVA in the foreground:

```shell
make build-fg
```

Once the OVA is built, it should be located at `./output/capv-haproxy.ova` and be around `240MiB`.

### Downloading the OVA

A full list of the published HAProxy load balancer images for CAPV may be obtained with the following command:

```shell
gsutil ls gs://capv-images/extra/haproxy/release/*
```

Or, to produce a list of URLs for the same image files (and their checksums), the following command may be used:

```shell
gsutil ls gs://capv-images/extra/haproxy/release/*/*.{ova,sha256} | sed 's~^gs://~http://storage.googleapis.com/~'
```
