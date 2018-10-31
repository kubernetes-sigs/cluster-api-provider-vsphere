# Build the manager binary
FROM golang:1.10.3 as builder

# Copy in the go src
WORKDIR /go/src/sigs.k8s.io/cluster-api-provider-vsphere
COPY pkg/    pkg/
COPY cmd/    cmd/
COPY vendor/ vendor/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager sigs.k8s.io/cluster-api-provider-vsphere/cmd/manager

# Copy the controller-manager into a thin image
FROM debian:stretch-slim
WORKDIR /root/

### BEGIN Legacy directives for the terraform variant
ENV TERRAFORM_VERSION=0.11.7
ENV TERRAFORM_ZIP=terraform_${TERRAFORM_VERSION}_linux_amd64.zip
ENV TERRAFORM_SHA256SUM=6b8ce67647a59b2a3f70199c304abca0ddec0e49fd060944c26f666298e23418
ENV TERRAFORM_SHAFILE=terraform_${TERRAFORM_VERSION}_SHA256SUMS

RUN apt-get update && apt-get install -y ca-certificates curl openssh-server unzip && \
    curl https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/${TERRAFORM_ZIP} > ${TERRAFORM_ZIP} && \
    echo "${TERRAFORM_SHA256SUM}  ${TERRAFORM_ZIP}" > ${TERRAFORM_SHAFILE} && \
    sha256sum --quiet -c ${TERRAFORM_SHAFILE} && \
    unzip ${TERRAFORM_ZIP} -d /bin && \
    rm -f ${TERRAFORM_ZIP} ${TERRAFORM_SHAFILE} && \
    rm -rf /var/lib/apt/lists/*

# Setup template provider
ENV TEMPLATE_PROVIDER_VERSION=1.0.0
ENV TEMPLATE_PROVIDER_ZIP=terraform-provider-template_${TEMPLATE_PROVIDER_VERSION}_linux_amd64.zip
ENV TEMPLATE_PROVIDER_SHA256SUM=f54c2764bd4d4c62c1c769681206dde7aa94b64b814a43cb05680f1ec8911977
ENV TEMPLATE_PROVIDER_SHAFILE=terraform-provider-template_${TEMPLATE_PROVIDER_VERSION}_SHA256SUMS

RUN curl https://releases.hashicorp.com/terraform-provider-template/${TEMPLATE_PROVIDER_VERSION}/${TEMPLATE_PROVIDER_ZIP} > ${TEMPLATE_PROVIDER_ZIP} && \
  echo "${TEMPLATE_PROVIDER_SHA256SUM}  ${TEMPLATE_PROVIDER_ZIP}" > ${TEMPLATE_PROVIDER_SHAFILE} && \
  sha256sum --quiet -c ${TEMPLATE_PROVIDER_SHAFILE} && \
  mkdir -p ~/.terraform.d/plugins/linux_amd64/ && \
  unzip ${TEMPLATE_PROVIDER_ZIP} -d ~/.terraform.d/plugins/linux_amd64/ && \
  rm -f ${TEMPLATE_PROVIDER_ZIP} ${TEMPLATE_PROVIDER_SHAFILE}

# Setup vsphere provider
ENV VSPHERE_PROVIDER_VERSION=1.5.0
ENV VSPHERE_PROVIDER_ZIP=terraform-provider-vsphere_${VSPHERE_PROVIDER_VERSION}_linux_amd64.zip
ENV VSPHERE_PROVIDER_SHA256SUM=6dd495feeb83aa8b098d4e9b0224b9e18b758153504449ff4ac2c6510ed4bb52
ENV VSPHERE_PROVIDER_SHAFILE=terraform-provider-vsphere_${VSPHERE_PROVIDER_VERSION}_SHA256SUMS

RUN curl https://releases.hashicorp.com/terraform-provider-vsphere/${VSPHERE_PROVIDER_VERSION}/${VSPHERE_PROVIDER_ZIP} > ${VSPHERE_PROVIDER_ZIP} && \
  echo "${VSPHERE_PROVIDER_SHA256SUM}  ${VSPHERE_PROVIDER_ZIP}" > ${VSPHERE_PROVIDER_SHAFILE} && \
  sha256sum --quiet -c ${VSPHERE_PROVIDER_SHAFILE} && \
  mkdir -p ~/.terraform.d/plugins/linux_amd64/ && \
  unzip ${VSPHERE_PROVIDER_ZIP} -d ~/.terraform.d/plugins/linux_amd64/ && \
  rm -f ${VSPHERE_PROVIDER_ZIP} ${VSPHERE_PROVIDER_SHAFILE}

### END Legacy directives for the terraform variant

COPY --from=builder /go/src/sigs.k8s.io/cluster-api-provider-vsphere/manager .
ENTRYPOINT ["./manager"]
