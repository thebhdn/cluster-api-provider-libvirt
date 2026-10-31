# Getting started

This guide installs the provider with `clusterctl` and creates a cluster. Prepare the libvirt host
first, see [host-setup.md](host-setup.md).

## Build the release files

Releases are built locally with `make`. There is no CI yet. The release series and contract are in
[`metadata.yaml`](../metadata.yaml) (`0.1`, `v1beta2`), so `VERSION` must be `v0.1.x`.

```sh
make release IMG=<registry>/cluster-api-provider-libvirt:v0.1.0 VERSION=v0.1.0
```

This builds and pushes the image, then writes the release files to `out/release/`:

| File | Content |
|---|---|
| `infrastructure-components.yaml` | CRDs, RBAC and the controller Deployment, using `IMG` |
| `metadata.yaml` | Release series for `clusterctl` |
| `cluster-template.yaml` | Default cluster flavor |

`make release-manifests` writes the same files without building the image. To publish a release,
attach these three files to a GitHub release with the tag `VERSION`.

## Install with clusterctl

To use the release files without a GitHub release, build a local `clusterctl` repository:

```sh
make clusterctl-repo IMG=<registry>/cluster-api-provider-libvirt:v0.1.0 VERSION=v0.1.0
clusterctl init --config out/repository/clusterctl.yaml --infrastructure libvirt:v0.1.0
```

The management cluster must be able to pull `IMG`. With kind, `kind load docker-image` works too.

For `qemu+tls://` or `qemu+libssh://`, set the credential env vars before `clusterctl init`, see
[host-setup.md](host-setup.md#credentials).

## Create a cluster

```sh
export LIBVIRT_URI=qemu+tcp://libvirt.lab/system
export LIBVIRT_BASE_POOL=capi-base-images
export LIBVIRT_DOMAIN_POOL=capi-vm-disks
export LIBVIRT_IMAGE=ubuntu-2404-kube-v1.34.1.qcow2
export CONTROL_PLANE_ENDPOINT_IP=192.168.122.10
export KUBERNETES_VERSION=v1.34.1

clusterctl generate cluster dev --config out/repository/clusterctl.yaml \
  --infrastructure libvirt:v0.1.0 --worker-machine-count 1 | kubectl apply -f -
```

Run `clusterctl generate cluster dev ... --list-variables` to see all variables and their defaults.

The template runs [kube-vip](https://kube-vip.io/) as a static pod on the control plane nodes to
hold `CONTROL_PLANE_ENDPOINT_IP`:

- `CONTROL_PLANE_ENDPOINT_IP` must be a free address in the libvirt network, outside the DHCP range.
- `KUBE_VIP_INTERFACE` (default `ens3`) must match the NIC name in the guest image.
- `LIBVIRT_IMAGE` must contain the kubeadm, kubelet and containerd versions for `KUBERNETES_VERSION`.
- No CNI is installed. Install one that matches `POD_CIDR` (default `10.244.0.0/16`).

The template is [`templates/cluster-template.yaml`](../templates/cluster-template.yaml). Smaller
examples with a single `Cluster` and `Machine` are in [`templates/`](../templates/).

## Provider ID

CAPI links a Node to its Machine by provider ID. The template already sets the kubelet
`provider-id`. If you write your own bootstrap config, set it the same way:

```yaml
kubeletExtraArgs:
  - name: provider-id
    value: "libvirt://{{ ds.meta_data.instance_id }}"
```

## Install from source

To install into a cluster that already runs CAPI, without `clusterctl`:

```sh
make docker-build docker-push IMG=<registry>/cluster-api-provider-libvirt:<tag>
make install                                                   # CRDs
make deploy IMG=<registry>/cluster-api-provider-libvirt:<tag>  # controller
```

`make build-installer IMG=...` writes a single `dist/install.yaml` with the CRDs and the deployment.

To remove it:

```sh
make undeploy
make uninstall
```
