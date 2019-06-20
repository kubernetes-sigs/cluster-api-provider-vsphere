FROM debian:stretch-slim

RUN apt-get update && apt-get -y install gettext curl

RUN curl -LO https://dl.google.com/go/go1.12.6.linux-amd64.tar.gz && \
       tar -xvf go1.12.6.linux-amd64.tar.gz && \
       mv go /usr/local

RUN curl -L -o kustomize "https://github.com/kubernetes-sigs/kustomize/releases/download/v2.0.0/kustomize_2.0.0_linux_amd64" && \
       chmod +x kustomize && \
       mv kustomize /usr/local/bin

ENV GOROOT=/usr/local/go
ENV GOPATH=/go
ENV PATH=$GOPATH/bin:$GOROOT/bin:$PATH
