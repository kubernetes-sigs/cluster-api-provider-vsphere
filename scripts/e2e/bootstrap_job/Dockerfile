FROM photon:2.0
LABEL maintainer="Hui Luo <luoh@vmware.com>"

RUN tdnf install -y iputils wget openssh

COPY *.sh /clusterapi/
COPY bin /clusterapi/bin
COPY spec /clusterapi/spec

WORKDIR /clusterapi/
CMD ["shell"]
ENTRYPOINT ["/clusterapi/clusterctl.sh"]
