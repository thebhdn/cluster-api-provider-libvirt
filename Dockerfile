# All stages use Debian 13 (trixie) so glibc matches between build, libraries and runtime.
ARG GO_IMAGE=golang:1.25-trixie
ARG LIBS_IMAGE=debian:trixie-slim
ARG RUNTIME_IMAGE=gcr.io/distroless/cc-debian13:nonroot

# Build the manager binary
FROM ${GO_IMAGE} AS builder
ARG TARGETOS
ARG TARGETARCH
# The Makefile passes GO_TAGS; libvirt_dlopen loads libvirt.so.0 at runtime instead of linking it.
ARG GO_TAGS=libvirt_dlopen

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# The CGO bindings need the libvirt C headers.
RUN apt-get update && apt-get install -y --no-install-recommends libvirt-dev && rm -rf /var/lib/apt/lists/*

# Copy the Go source (relies on .dockerignore to filter)
COPY . .

# Build
# the GOARCH has no default value to allow the binary to be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
RUN CGO_ENABLED=1 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -tags="${GO_TAGS}" -trimpath -ldflags="-s -w" -a -o manager cmd/main.go

# Collect libvirt.so.0 and its shared library dependencies into /out
FROM ${LIBS_IMAGE} AS libs
RUN apt-get update && apt-get install -y --no-install-recommends libvirt0 && rm -rf /var/lib/apt/lists/*
# The runtime image already ships glibc, libgcc and libstdc++, so those are skipped.
RUN set -eu; \
    libvirt="$(ldconfig -p | awk '$1 == "libvirt.so.0" { print $NF; exit }')"; \
    test -n "$libvirt"; \
    for lib in "$libvirt" $(ldd "$libvirt" | awk '$2 == "=>" && $3 ~ /^\// { print $3 }'); do \
        case "$(basename "$lib")" in \
            libc.so.*|libm.so.*|libgcc_s.so.*|libstdc++.so.*|ld-linux*) continue ;; \
        esac; \
        dir="/out$(dirname "$(readlink -f "$lib")")"; \
        mkdir -p "$dir"; \
        cp -L "$lib" "$dir/"; \
    done; \
    ! ldd "$libvirt" | grep -q "not found"

FROM ${RUNTIME_IMAGE}
WORKDIR /
COPY --from=libs /out/ /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
