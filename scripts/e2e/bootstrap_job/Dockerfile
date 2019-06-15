FROM photon:2.0
LABEL maintainer="Hui Luo <luoh@vmware.com>"

RUN tdnf install -y iputils wget openssh gettext

COPY *.sh /clusterapi/
COPY spec /clusterapi/spec

WORKDIR /clusterapi/
CMD ["shell"]
ENTRYPOINT ["/clusterapi/clusterctl.sh"]
