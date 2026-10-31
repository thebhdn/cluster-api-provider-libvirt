# cluster-api-provider-libvirt

A [Cluster API](https://cluster-api.sigs.k8s.io/) (CAPI) infrastructure provider that runs Kubernetes
machines as KVM virtual machines on a [libvirt](https://libvirt.org/) host.

> **Status:** proof of concept for development and testing, not for production. All machines of a
> cluster run on one libvirt host, and failure domains are not supported. The API is `v1alpha1`
> and may change.

```
Management cluster                             libvirt host
┌────────────────────────────────┐             ┌──────────────────────────────────┐
│ CAPI core + bootstrap provider │             │ network     (DHCP, e.g. NAT)     │
│ cluster-api-provider-libvirt   │──libvirt──> │ basePool    (base images, qcow2) │
│   LibvirtCluster controller    │     API     │ domainPool  (VM disks, ISOs)     │
│   LibvirtMachine controller    │             │ domains     (one per machine)    │
└────────────────────────────────┘             └──────────────────────────────────┘
```

You prepare a libvirt host with a network, storage pools and a base image. The provider creates a
VM for every CAPI `Machine`, boots it with the bootstrap data through cloud-init, and cleans it up
when the `Machine` is deleted.

## Quickstart

1. Build a base image with [image-builder](https://github.com/kubernetes-sigs/image-builder) and
   prepare the libvirt host (network, pools, image upload), see [docs/host-setup.md](docs/host-setup.md):

   ```sh
   hack/prepare-host.sh --image ubuntu-2404-kube-v1.34.1.qcow2
   ```

2. Build the release files and install the provider into a management cluster:

   ```sh
   make clusterctl-repo IMG=<registry>/cluster-api-provider-libvirt:v0.1.0
   clusterctl init --config out/repository/clusterctl.yaml --infrastructure libvirt:v0.1.0
   ```

3. Create a cluster:

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

The full steps, all template variables and the kube-vip notes are in
[docs/getting-started.md](docs/getting-started.md).

## Documentation

| Document | Content |
|---|---|
| [Host setup](docs/host-setup.md) | Connection URI and credentials, network, storage pools and base image |
| [Getting started](docs/getting-started.md) | Release files, `clusterctl`, cluster template, install from source |
| [API](docs/api.md) | Fields, defaults, validation and `kubectl` output |
| [How it works](docs/how-it-works.md) | Reconcile flow, names and identity, conditions |
| [Development](docs/development.md) | Requirements, image build, layout, make targets, tests |

## Known limitations

- One libvirt host per cluster. Failure domains are not supported.
- Networks and storage pools are not created or deleted by the provider.
- VMs boot from a prepared base image in `basePool`. Images cannot be loaded from a URL yet.
- One NIC per VM, on a libvirt network with DHCP. Static IPs are not supported yet.
- The control plane endpoint IP is chosen by the user. The default template runs kube-vip to hold it.

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
