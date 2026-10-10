# cluster-api-provider-libvirt

A [Cluster API](https://cluster-api.sigs.k8s.io/) (CAPI) infrastructure provider that runs Kubernetes
machines as KVM virtual machines on a [libvirt](https://libvirt.org/) host.

It implements the CAPI `v1beta2` infrastructure contract with two resources in the
`infrastructure.cluster.x-k8s.io/v1alpha1` API group:

| Kind | Purpose |
|---|---|
| `LibvirtCluster` | Points to a libvirt host and the network and storage pools the machines use |
| `LibvirtMachine` | One VM: CPU, memory, disk size and the base image to boot from |

`LibvirtClusterTemplate` and `LibvirtMachineTemplate` are also available, for use with ClusterClass
and MachineDeployments.

> **Status:** proof of concept. The API is `v1alpha1` and may change.

## How it works

The provider is a consumer of an existing libvirt host. An admin prepares the network, the storage
pools and the base images. The provider checks that they exist and creates VMs on top of them.

```
Management cluster                          libvirt host
┌───────────────────────────────┐           ┌──────────────────────────────────┐
│ CAPI core + bootstrap provider│           │ network     (DHCP, e.g. NAT)     │
│ cluster-api-provider-libvirt ─┼─ libvirt ─┼▶ basePool   (base images, qcow2) │
│   LibvirtCluster controller   │    API    │  domainPool (VM disks, ISOs)     │
│   LibvirtMachine controller   │           │  domains    (one per machine)    │
└───────────────────────────────┘           └──────────────────────────────────┘
```

### LibvirtCluster

1. Waits until CAPI sets the owner `Cluster`, then adds a finalizer.
2. Checks that a control plane endpoint is set, either on `LibvirtCluster.spec.controlPlaneEndpoint`
   or on `Cluster.spec.controlPlaneEndpoint`. If not, it sets `ControlPlaneEndpointReady=False` and waits.
3. Connects to `spec.uri` and checks that `spec.network`, `spec.basePool` and `spec.domainPool`
   exist and are active. It does not create or start them.
4. Sets `status.initialization.provisioned=true` when everything is ready.

On delete, it only removes the finalizer. Nothing on the host is changed.

### LibvirtMachine

1. Waits for the owner `Machine`, the `Cluster`, the cluster infrastructure and the bootstrap
   data secret (`Machine.spec.bootstrap.dataSecretName`).
2. If the domain does not exist, it:
   - creates a root disk in `domainPool` as a qcow2 overlay on top of `spec.image` from `basePool`
   - builds a cloud-init NoCloud ISO (`user-data` from the bootstrap secret, `meta-data` with
     `instance-id` and `local-hostname`) and uploads it to `domainPool`
   - reserves a fixed IP for the VM in the network's DHCP configuration
   - defines the domain, turns on autostart and starts it
3. If the domain is stopped, it starts it.
4. If the domain is running, it reads the VM address from the network's DHCP leases into
   `status.addresses`.

On delete, it removes the domain, the root disk, the cloud-init ISO and the DHCP reservation.

### Names and identity

| Item | Value |
|---|---|
| Domain name | `<namespace>-<name>-<hash>`, for example `default-libvirt-xd-0-3fa91c`. Unique on the host and a valid DNS label |
| Guest hostname | The `LibvirtMachine` name |
| Domain UUID | The `LibvirtMachine` UID |
| Provider ID | `libvirt://<LibvirtMachine UID>` |
| cloud-init `instance-id` | The domain UUID |
| Volumes | `<domain name>.qcow2` and `cloudinit-<domain name>.iso` in `domainPool` |
| MAC address | Derived from the domain name, with the `52:54:00` prefix |

CAPI links a Node to its Machine by provider ID. Set the kubelet `provider-id` in your bootstrap
config so the two match:

```yaml
kubeletExtraArgs:
  - name: provider-id
    value: "libvirt://{{ ds.meta_data.instance_id }}"
```

### Conditions

| Resource | Condition | Meaning |
|---|---|---|
| `LibvirtCluster` | `ControlPlaneEndpointReady` | A control plane endpoint is set |
| `LibvirtCluster` | `InfrastructureReady` | Network and pools exist and are active (reasons: `InfrastructureMissing`, `InfrastructureInactive`, `InfrastructureProvisioningFailed`) |
| `LibvirtMachine` | `InfrastructureReady` | The cluster infrastructure is provisioned |
| `LibvirtMachine` | `DomainProvisioningReady` | The domain is being created or creation failed |
| `LibvirtMachine` | `MachineCreated` | The domain was created |
| `LibvirtMachine` | `DomainRunning` | The domain is running |

## Prepare the libvirt host

The provider needs a libvirt host reachable from the management cluster, for example over
`qemu+tcp://`, `qemu+tls://` or `qemu+ssh://`.

1. **Network:** a libvirt network with an IPv4 subnet and DHCP, for example the `default` NAT network.
   The provider reserves VM addresses inside the DHCP range, or in the last 200 addresses of the
   subnet when no range is set.
2. **Storage pools:** two active pools, one for base images (`basePool`) and one for VM disks
   (`domainPool`). They can be the same pool.
3. **Base image:** a cloud image with cloud-init and the Kubernetes tools you need, uploaded to
   `basePool` as a qcow2 volume.

```sh
virsh pool-define-as capi-base-images dir --target /var/lib/libvirt/capi/base
virsh pool-define-as capi-vm-disks dir --target /var/lib/libvirt/capi/disks
for pool in capi-base-images capi-vm-disks; do
  virsh pool-build $pool && virsh pool-start $pool && virsh pool-autostart $pool
done

cp ubuntu-noble-base.qcow2 /var/lib/libvirt/capi/base/
virsh pool-refresh capi-base-images
```

## Usage

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: LibvirtCluster
metadata:
  name: libvirt-xd
spec:
  uri: qemu+tcp://libvirt.lab/system
  network: libvirt-lab
  basePool: capi-base-images
  domainPool: capi-vm-disks
  controlPlaneEndpoint:
    host: 192.168.122.10 # a free address outside the DHCP range, e.g. a kube-vip VIP
    port: 6443
---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: LibvirtMachine
metadata:
  name: libvirt-xd-0
spec:
  vcpu: 2          # default 1
  memoryMiB: 2048  # default 1024
  diskGiB: 20      # default 20
  image: ubuntu-noble-base.qcow2
```

Full examples with the matching CAPI `Cluster` and `Machine` are in [`templates/`](templates/).

## Install

The provider is listed for `clusterctl` in [`metadata.yaml`](metadata.yaml) (release series `0.1`,
contract `v1beta2`). To install from source into a cluster that already runs CAPI:

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

## Development

### Requirements

- Go 1.25 or newer
- libvirt development headers (`libvirt-dev` on Debian/Ubuntu, `libvirt` on Arch), because the
  provider uses the official CGO bindings [`libvirt.org/go/libvirt`](https://pkg.go.dev/libvirt.org/go/libvirt)
- Docker, for image builds

The container image builds with the `libvirt_dlopen` tag (`GO_TAGS` in the [`Makefile`](Makefile)).
In that mode, the binary loads `libvirt.so.0` at runtime, so the runtime image ships the libvirt
client library.

### Layout

| Path | Content |
|---|---|
| `api/v1alpha1/` | CRD types, finalizers and condition constants |
| `cmd/main.go` | Manager entry point |
| `internal/controller/` | `LibvirtCluster` and `LibvirtMachine` reconcilers, and a mock provider for tests |
| `internal/libvirtclient/` | All libvirt calls: infra checks, domains, volumes, cloud-init ISO, DHCP reservations |
| `internal/libvirtclient/builders/` | Typed builders for domain and volume XML |
| `templates/` | Example manifests |

### Common tasks

```sh
make help       # list all targets
make manifests  # regenerate CRDs and RBAC
make generate   # regenerate DeepCopy code
make test       # unit and controller tests (envtest)
make lint       # golangci-lint
make run        # run the controller against your current kubeconfig
```

Controller tests use a mock provider, so they do not need a libvirt host. A few `libvirtclient`
tests skip themselves when libvirt is not available.

## Known limitations

- Networks and storage pools are not created or deleted by the provider.
- VMs boot from a prepared base image in `basePool`. Images cannot be loaded from a URL yet.
- One NIC per VM, on a libvirt network with DHCP. Static IPs are not supported yet.
- The control plane endpoint must be set by the user. No VIP is allocated automatically.

## License

Copyright 2026 Bohdan Leshchenko.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
