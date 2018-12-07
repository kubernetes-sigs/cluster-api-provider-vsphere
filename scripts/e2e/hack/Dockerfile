# build from test-infra prow image
FROM gcr.io/k8s-testimages/kubekins-e2e:v20181120-dea0825e3-master

ADD cluster-api-provider-vsphere /go/src/sigs.k8s.io/cluster-api-provider-vsphere/

RUN chmod +x /go/src/sigs.k8s.io/cluster-api-provider-vsphere/scripts/e2e/e2e.sh
WORKDIR /go/src/sigs.k8s.io/cluster-api-provider-vsphere
CMD ["shell"]
ENTRYPOINT ["./scripts/e2e/e2e.sh"]
