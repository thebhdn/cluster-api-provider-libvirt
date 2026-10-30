# Prepare the libvirt host

The provider is a consumer of an existing libvirt host. You prepare the network, the storage pools
and the base images. The provider checks that they exist and creates VMs on top of them. It does
not create, change or delete networks or pools.

## Connection

The host must be reachable from the management cluster, for example over `qemu+tls://`,
`qemu+tcp://` or `qemu+libssh://`. The controller image is distroless and has no `ssh` binary, so
use `qemu+libssh://` or `qemu+libssh2://` instead of `qemu+ssh://`.

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
virsh pool-define-as capi-base-images dir --target /var/lib/libvirt/capi/base
virsh pool-define-as capi-vm-disks dir --target /var/lib/libvirt/capi/disks
for pool in capi-base-images capi-vm-disks; do
  virsh pool-build $pool && virsh pool-start $pool && virsh pool-autostart $pool
done
```

## Base image

A cloud image with cloud-init, containerd, kubeadm and kubelet for the Kubernetes version you want.
Upload it to the base pool as a qcow2 volume:

```sh
cp ubuntu-noble-base.qcow2 /var/lib/libvirt/capi/base/
virsh pool-refresh capi-base-images
```

`LibvirtMachine.spec.image` is the volume name, here `ubuntu-noble-base.qcow2`. Images cannot be
loaded from a URL yet.
