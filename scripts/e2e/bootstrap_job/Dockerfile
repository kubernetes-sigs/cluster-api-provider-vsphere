FROM photon:2.0
LABEL maintainer="Hui Luo <luoh@vmware.com>"

RUN tdnf install -y iputils wget openssh
RUN wget https://storage.googleapis.com/kubernetes-release/release/v1.10.2/bin/linux/amd64/kubectl --no-verbose -O /usr/local/bin/kubectl
RUN chmod +x /usr/local/bin/kubectl

COPY *.sh /clusterapi/
COPY spec /clusterapi/spec

WORKDIR /clusterapi/
CMD ["shell"]
ENTRYPOINT ["/clusterapi/clusterctl.sh"]
