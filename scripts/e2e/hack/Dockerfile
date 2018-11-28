# build from test-infra prow image
FROM gcr.io/k8s-testimages/kubekins-e2e:v20181120-dea0825e3-master

COPY *.sh /ci/
COPY *.template /ci/
COPY *.yml /ci/

RUN chmod +x /ci/e2e.sh
WORKDIR /ci/
CMD ["shell"]
ENTRYPOINT ["/ci/e2e.sh"]
