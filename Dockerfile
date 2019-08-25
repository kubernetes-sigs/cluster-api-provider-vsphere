################################################################################
##                               BUILD ARGS                                   ##
################################################################################
# This build arg allows the specification of a custom Golang image.
ARG GOLANG_IMAGE=golang:1.12.6

# The distroless image on which the CAPV manager image is built.
#
# Please do not use "latest". Explicit tags should be used to provide
# deterministic builds. This image doesn't have semantic version tags, but
# the fully-qualified image can be obtained by entering
# "gcr.io/distroless/static:latest" in a browser and then copying the
# fully-qualified image from the web page.
ARG DISTROLESS_IMAGE=gcr.io/distroless/static@sha256:48e0d165f07d499c02732d924e84efbc73df8021b12c24940e18a9306589430e

################################################################################
##                              BUILD STAGE                                   ##
################################################################################
# Build the manager as a statically compiled binary so it has no dependencies
# libc, muscl, etc. This allows the binary to be shipped inside of a static,
# distroless container image.
FROM ${GOLANG_IMAGE} as builder
WORKDIR /build
COPY go.mod go.sum main.go ./
COPY api/         api/
COPY controllers/ controllers/
COPY pkg/         pkg/
COPY vendor/      vendor/
ENV CGO_ENABLED=0
RUN go build -mod=vendor -a -ldflags='-w -s -extldflags="static"' -o manager .

################################################################################
##                               MAIN STAGE                                   ##
################################################################################
# Copy the manager into the distroless image.
FROM ${DISTROLESS_IMAGE}
LABEL "maintainer" "Andrew Kutz <akutz@vmware.com>"
COPY --from=builder /build/manager /manager
ENTRYPOINT [ "/manager" ]
