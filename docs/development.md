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
| `hack/` | Boilerplate and the local clusterctl config template |
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

## Releases

See [getting-started.md](getting-started.md#build-the-release-files) for `make release`,
`make release-manifests` and `make clusterctl-repo`.
