# Development

## Requirements

- Go 1.25 or newer
- libvirt development headers (`libvirt-dev` on Debian/Ubuntu, `libvirt` on Arch), because the
  provider uses the official CGO bindings [`libvirt.org/go/libvirt`](https://pkg.go.dev/libvirt.org/go/libvirt)
- Docker, for image builds

## Container image

The image builds with the `libvirt_dlopen` tag (`GO_TAGS` in the [`Makefile`](../Makefile)). In
that mode, the binary loads `libvirt.so.0` at runtime. The runtime image is
`gcr.io/distroless/cc-debian13:nonroot` plus `libvirt.so.0` and the shared libraries it needs,
copied from `debian:trixie-slim`. All stages use Debian 13, so the glibc versions match.

## Layout

| Path | Content |
|---|---|
| `api/v1alpha1/` | CRD types, validation markers, finalizers and condition constants |
| `cmd/main.go` | Manager entry point |
| `internal/controller/` | `LibvirtCluster` and `LibvirtMachine` reconcilers, and a mock provider for tests |
| `internal/libvirtclient/` | All libvirt calls: infra checks, domains, volumes, cloud-init ISO, DHCP reservations |
| `internal/libvirtclient/builders/` | Typed builders for domain and volume XML |
| `templates/` | Cluster template and example manifests |
| `hack/` | Boilerplate, the local clusterctl config template and `prepare-host.sh` |
| `Tiltfile` | Local development loop with Tilt |
| `docs/` | This documentation |

## Common tasks

```sh
make help       # list all targets
make manifests  # regenerate CRDs and RBAC
make generate   # regenerate DeepCopy code
make test       # unit and controller tests (envtest)
make lint       # golangci-lint
make run        # run the controller against your current kubeconfig
```

## Tests

Controller tests use a mock provider, so they do not need a libvirt host. API validation tests run
against the envtest API server with the generated CRDs. A few `libvirtclient` tests skip themselves
when libvirt is not available.

## Tilt

[Tilt](https://tilt.dev) rebuilds and redeploys the controller on every code change. The
[`Tiltfile`](../Tiltfile) is in this repo, not in the CAPI repo: CAPI's Tiltfile builds providers
without cgo, and the libvirt bindings need cgo. Tilt builds the normal [`Dockerfile`](../Dockerfile),
where a Go build cache keeps rebuilds short.

Requirements: `tilt`, `kind`, `clusterctl`, `kubectl` and Docker.

```sh
kind create cluster --name capi-libvirt
tilt up
```

Tilt then:

1. Installs CAPI core, kubeadm bootstrap and kubeadm control plane with `clusterctl init`, at the
   CAPI version from `go.mod`. It skips this when CAPI is already installed.
2. Builds the controller image, loads it into kind and deploys `config/default`.
3. Rebuilds and restarts the controller when a Go file (not `_test.go`), `go.mod` or `go.sum`
   changes. The files come from `.dockerignore`, so docs and tests do not trigger a rebuild.
4. Offers a `cluster` button that applies `templates/cluster-template.yaml`.

Optional settings go in `tilt-settings.yaml` (ignored by git):

```yaml
install_capi: true          # false if you manage CAPI yourself
allowed_contexts: []        # extra non-local kube contexts
cluster_name: dev
cluster_namespace: default
cluster_template_vars:      # variables for the cluster button
  LIBVIRT_URI: qemu+tcp://172.18.0.1/system
  LIBVIRT_BASE_POOL: capi-base-images
  LIBVIRT_DOMAIN_POOL: capi-vm-disks
  LIBVIRT_IMAGE: ubuntu-2404-kube-v1.34.1.qcow2
  CONTROL_PLANE_ENDPOINT_IP: 192.168.122.10
  KUBERNETES_VERSION: v1.34.1
  WORKER_MACHINE_COUNT: 1
```

The controller runs inside kind, so `qemu:///system` does not work: it must reach libvirt over the
network. On Linux, the host is the gateway of the `kind` Docker network
(`docker network inspect kind`, often `172.18.0.1`), and libvirt must listen there, for example
with `virtproxyd-tcp.socket` in a closed lab. For TLS or SSH, create the controller-wide Secret by
hand, because `config/default` does not include it:

```sh
kubectl create secret generic cluster-api-provider-libvirt-credentials \
  -n cluster-api-provider-libvirt-system \
  --from-file=ca.crt=cacert.pem --from-file=tls.crt=clientcert.pem --from-file=tls.key=clientkey.pem
```

## Releases

See [getting-started.md](getting-started.md#build-the-release-files) for `make release`,
`make release-manifests` and `make clusterctl-repo`.
