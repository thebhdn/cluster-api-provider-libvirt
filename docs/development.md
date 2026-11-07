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
| `test/e2e/` | E2E suite, its config and test data |
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

## E2E tests

The e2e suite in [`test/e2e/`](../test/e2e/) uses the
[CAPI test framework](https://pkg.go.dev/sigs.k8s.io/cluster-api/test/framework) and needs a real
libvirt host. It:

1. Builds the controller image and creates a kind management cluster.
2. Installs CAPI core, kubeadm bootstrap and kubeadm control plane (the CAPI version from `go.mod`)
   and this provider from `config/release`.
3. Runs the upstream `MachineDeploymentScaleSpec`: creates a cluster with 1 control plane and 1
   worker, scales the workers to 3 and back to 1, checks that the cluster and machines are ready,
   then deletes the cluster.
4. Checks that no domains, volumes in `domainPool` or DHCP host entries for the test namespace
   are left on the host.

The cluster template is [`templates/cluster-template.yaml`](../templates/cluster-template.yaml)
plus a `ClusterResourceSet` that installs kindnet as the CNI, see
[`test/e2e/data/`](../test/e2e/data/).

Requirements: Docker, `virsh` access to the host, and a host prepared with
[`hack/prepare-host.sh`](../hack/prepare-host.sh) with an image for `KUBERNETES_VERSION`. At peak
it runs 1 control plane (4 GiB) and 3 workers (2 GiB each).

```sh
export LIBVIRT_URI=qemu+tcp://172.18.0.1/system      # as seen from kind, see the Tilt section
export LIBVIRT_HOST_URI=qemu:///system               # as seen from the test, default: LIBVIRT_URI
export LIBVIRT_IMAGE=ubuntu-2404-kube-v1.34.1.qcow2
export KUBERNETES_VERSION=v1.34.1
export CONTROL_PLANE_ENDPOINT_IP=192.168.122.10      # free, outside the DHCP range
make test-e2e
```

All variables and their defaults are in [`test/e2e/config/libvirt.yaml`](../test/e2e/config/libvirt.yaml).
Every variable there can be set as an env var. For TLS or SSH, also set the credential env vars
from [host-setup.md](host-setup.md#credentials).

The CAPI controllers in kind must reach `CONTROL_PLANE_ENDPOINT_IP`. A libvirt NAT network only
accepts new connections from the host itself, not traffic forwarded from the kind network, so you
may need a `route` or `open` forward mode network for the test.

Logs and resource dumps go to `out/e2e/artifacts/`. Useful options:

```sh
make test-e2e E2E_ARGS=-e2e.skip-resource-cleanup   # keep the clusters to debug
make test-e2e E2E_ARGS=-e2e.use-existing-cluster    # use the current kubeconfig, not a new kind cluster
```

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
