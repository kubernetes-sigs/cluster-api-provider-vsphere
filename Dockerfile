# Build the manager binary
FROM blueshift-docker-local.artifactory.eng.vmware.com/utils/golang:1.10.3 as builder

# Copy in the go src
WORKDIR /go/src/sigs.k8s.io/cluster-api-provider-vsphere
COPY pkg/    pkg/
COPY cmd/    cmd/
COPY vendor/ vendor/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager sigs.k8s.io/cluster-api-provider-vsphere/cmd/manager

# Copy the controller-manager into a thin image
FROM blueshift-docker-local.artifactory.eng.vmware.com/utils/ubuntu:xenial-20181113
WORKDIR /root/

RUN apt-get update && apt-get install -y ca-certificates openssh-client && rm -rf /var/lib/apt/lists/*

COPY --from=builder /go/src/sigs.k8s.io/cluster-api-provider-vsphere/manager .
ENTRYPOINT ["./manager"]
