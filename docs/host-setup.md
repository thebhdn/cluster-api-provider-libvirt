# Prepare the libvirt host

The provider is a consumer of an existing libvirt host. You prepare the network, the storage pools
and the base images. The provider checks that they exist and creates VMs on top of them. It does
not create, change or delete networks or pools.

## Quick setup

[`hack/prepare-host.sh`](../hack/prepare-host.sh) does the steps below over any libvirt URI. It
checks the network, creates and starts the pools, and uploads a base image. Running it again
changes nothing that is already in place.

```sh
export LIBVIRT_URI=qemu+tls://libvirt.lab/system   # default: qemu:///system
hack/prepare-host.sh --image output/ubuntu-2404-kube-v1.34.1/ubuntu-2404-kube-v1.34.1 \
  --image-name ubuntu-2404-kube-v1.34.1.qcow2
```

It reads the same variables as the cluster template (`LIBVIRT_URI`, `LIBVIRT_NETWORK`,
`LIBVIRT_BASE_POOL`, `LIBVIRT_DOMAIN_POOL`, `LIBVIRT_IMAGE`), so one set of `export`s works for both.
New pools go to `/var/lib/libvirt/capi/<pool>` on the host (`--pool-root`). `--force` replaces an
existing image. See `hack/prepare-host.sh --help`.

The client running the script needs only `virsh`. The image is streamed over the libvirt
connection, so no SSH or file copy to the host is needed.

## Connection

The host must be reachable from the management cluster, for example over `qemu+tls://`,
`qemu+tcp://` or `qemu+libssh://`. The controller image is distroless and has no `ssh` binary, so
use `qemu+libssh://` or `qemu+libssh2://` instead of `qemu+ssh://`.

`qemu+tcp://` without auth needs no credentials, but anyone who can reach the port controls the
host. Use it only in a closed lab.

### Credentials

For TLS and SSH, the controller needs client credentials. Like the Proxmox provider, they come
from one controller-wide Secret, filled at `clusterctl init`. A `LibvirtCluster` can override it
with its own Secret.

**Controller-wide** (`cluster-api-provider-libvirt-credentials` in the controller namespace): set
base64 env vars before `clusterctl init`. Unset vars stay empty, and an empty Secret means a plain
connection, so `qemu+tcp://` needs none of them.

| Env var | Secret key | For |
|---|---|---|
| `LIBVIRT_TLS_CA_B64` | `ca.crt` | TLS: CA that signed the libvirt server certificate |
| `LIBVIRT_TLS_CERT_B64` | `tls.crt` | TLS: client certificate |
| `LIBVIRT_TLS_KEY_B64` | `tls.key` | TLS: client key |
| `LIBVIRT_SSH_PRIVATE_KEY_B64` | `ssh-privatekey` | SSH: private key |
| `LIBVIRT_SSH_KNOWN_HOSTS_B64` | `known_hosts` | SSH: `known_hosts` with the libvirt host key |

TLS (`qemu+tls://`):

```sh
export LIBVIRT_TLS_CA_B64=$(base64 -w0 cacert.pem)
export LIBVIRT_TLS_CERT_B64=$(base64 -w0 clientcert.pem)
export LIBVIRT_TLS_KEY_B64=$(base64 -w0 clientkey.pem)
clusterctl init --config out/repository/clusterctl.yaml --infrastructure libvirt:v0.1.0
```

SSH (`qemu+libssh://` or `qemu+libssh2://`). Put the user in the URI, for example
`qemu+libssh://capi@libvirt.lab/system`:

```sh
export LIBVIRT_SSH_PRIVATE_KEY_B64=$(base64 -w0 id_ed25519)
export LIBVIRT_SSH_KNOWN_HOSTS_B64=$(ssh-keyscan libvirt.lab 2>/dev/null | base64 -w0)
clusterctl init --config out/repository/clusterctl.yaml --infrastructure libvirt:v0.1.0
```

To change the credentials later, edit the Secret. The controller reads it again for every
connection, so no restart is needed.

**Per cluster** (optional): create a Secret with the same keys in the namespace of the
`LibvirtCluster` and set `spec.credentialsRef`. It replaces the controller-wide Secret for that
cluster.

```sh
kubectl create secret generic libvirt-credentials \
  --from-file=ca.crt=cacert.pem \
  --from-file=tls.crt=clientcert.pem \
  --from-file=tls.key=clientkey.pem
```

```yaml
spec:
  uri: qemu+tls://libvirt.lab/system
  credentialsRef:
    name: libvirt-credentials
```

The TLS keys are the same as in a `kubernetes.io/tls` Secret plus `ca.crt`, so a cert-manager
`Certificate` Secret works as is.

The controller writes the files to a private temp dir only while it opens a connection, and adds
`pkipath=` (TLS) or `keyfile=`, `known_hosts=` and `sshauth=privkey` (SSH) to the URI. If a Secret
has only some of the TLS or SSH keys, or a `credentialsRef` Secret is missing,
`InfrastructureReady` is `False` with reason `CredentialsUnavailable`. `make deploy` does not
create the controller-wide Secret; create it by hand with the same name if you need it.

## Network

A libvirt network with an IPv4 subnet and DHCP, for example the `default` NAT network.

The provider reserves a fixed address for every VM in the network's DHCP configuration. It picks
addresses inside the DHCP range, or in the last 200 addresses of the subnet when no range is set.

Keep a free address outside the DHCP range for the control plane endpoint.

## Storage pools

Two active pools:

| Pool | Field | Content |
|---|---|---|
| Base pool | `basePool` | Base images (qcow2) |
| Domain pool | `domainPool` | VM root disks and cloud-init ISOs |

They can be the same pool.

```sh
virsh pool-define-as capi-base-images dir --target /var/lib/libvirt/capi/capi-base-images
virsh pool-define-as capi-vm-disks dir --target /var/lib/libvirt/capi/capi-vm-disks
for pool in capi-base-images capi-vm-disks; do
  virsh pool-build $pool && virsh pool-start $pool && virsh pool-autostart $pool
done
```

## Base image

A qcow2 image with cloud-init, containerd, kubeadm and kubelet for the Kubernetes version you want.
Like the Proxmox and OpenStack providers, the provider does not build images. Build one with
[image-builder](https://github.com/kubernetes-sigs/image-builder), using its QEMU target:

```sh
git clone https://github.com/kubernetes-sigs/image-builder.git
cd image-builder/images/capi
make deps-qemu
make build-qemu-ubuntu-2404
```

The image is written to `output/ubuntu-2404-kube-<version>/`. To pick the Kubernetes version, pass
a Packer variables file with `PACKER_VAR_FILES`, see the image-builder documentation.

Upload it with the script above, or by hand on the host:

```sh
cp ubuntu-2404-kube-v1.34.1.qcow2 /var/lib/libvirt/capi/capi-base-images/
virsh pool-refresh capi-base-images
```

`LibvirtMachine.spec.image` is the volume name, here `ubuntu-2404-kube-v1.34.1.qcow2`. Every VM disk
is a qcow2 overlay on top of it, so do not change or delete the image while machines use it. For a
new Kubernetes version, upload a new image under a new name and point a new `LibvirtMachineTemplate`
to it.

Check the NIC name in the guest (`ip link`). The default template runs kube-vip on `ens3`
(`KUBE_VIP_INTERFACE`). Images cannot be loaded from a URL yet.
