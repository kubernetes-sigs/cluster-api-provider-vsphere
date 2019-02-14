FROM photon:2.0 as ci

ENV GOVERSION=1.10.5
ENV PATH=$PATH:/usr/local/go/bin

RUN set -eux; \
    tdnf remove -y toybox; \
    tdnf install -y make tar gzip python2 python-pip sed git diffutils findutils rpm \
    gawk util-linux which docker gptfdisk e2fsprogs grub2-efi efibootmgr parted dosfstools xz docker; \
    curl -L'#' -k https://storage.googleapis.com/golang/go$GOVERSION.linux-amd64.tar.gz | tar xzf - -C /usr/local; \
    curl -o /usr/bin/jq -L'#' -k https://github.com/stedolan/jq/releases/download/jq-1.5/jq-linux64 && chmod +x /usr/bin/jq;

RUN set -eux; \
    curl -OL'#' https://storage.googleapis.com/vic-product-ova-build-deps/qemu-img.xz; \
    xz -d qemu-img.xz; \
    chmod +x qemu-img; \
    mv qemu-img /usr/bin/qemu-img;

ADD installer /go/src/sigs.k8s.io/cluster-api-provider-vsphere/installer

WORKDIR /go/src/sigs.k8s.io/cluster-api-provider-vsphere/installer
ENTRYPOINT ["./build/build.sh"]

FROM ci
RUN rm -rf /go/src/sigs.k8s.io/cluster-api-provider-vsphere/installer
WORKDIR /
ENTRYPOINT ["/bin/bash"]
