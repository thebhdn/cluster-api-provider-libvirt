# Build the manager binary
FROM golang:1.25 AS builder
ARG TARGETOS
ARG TARGETARCH
# go-libvirt is built with the libvirt_dlopen tag (passed by the Makefile
# as GO_TAGS - single source of truth). The C wrapper dlopens libvirt.so.0
# at RUNTIME, so CGO is required at build time and the libvirt shared
# library must be present in the runtime image (see runtime stage below).
ARG GO_TAGS=libvirt_dlopen

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# CGO build of go-libvirt needs the libvirt C headers (libvirt.h) and the
# shared library to link against.
RUN apt-get update && apt-get install -y --no-install-recommends libvirt-dev && rm -rf /var/lib/apt/lists/*

# Copy the Go source (relies on .dockerignore to filter)
COPY . .

# Build
# the GOARCH has no default value to allow the binary to be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
RUN CGO_ENABLED=1 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -tags="${GO_TAGS}" -a -o manager cmd/main.go

# The manager dlopens libvirt.so.0 at runtime (go-libvirt libvirt_dlopen
# tag), so the runtime image must provide glibc plus the libvirt shared
# library - distroless/static is not usable here.
FROM debian:stable-slim AS runtime
RUN apt-get update && apt-get install -y --no-install-recommends libvirt0 && rm -rf /var/lib/apt/lists/*
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
