# Build the manager binary
FROM golang:1.12 as builder

# Copy in the go src
WORKDIR /build
COPY go.mod go.sum ./
COPY pkg/    pkg/
COPY cmd/    cmd/
COPY vendor/ vendor/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod vendor -a -o manager ./cmd/manager

# Copy the controller-manager into a thin image
FROM debian:stretch-slim
WORKDIR /root/

RUN apt-get update && apt-get install -y ca-certificates openssh-client

COPY --from=builder /build/manager .
ENTRYPOINT ["./manager"]
