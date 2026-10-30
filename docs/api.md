# API

All resources are in the `infrastructure.cluster.x-k8s.io/v1alpha1` API group and follow the CAPI
`v1beta2` infrastructure contract.

| Kind | Purpose |
|---|---|
| `LibvirtCluster` | Points to a libvirt host and the network and storage pools the machines use |
| `LibvirtMachine` | One VM: CPU, memory, disk size and the base image to boot from |
| `LibvirtClusterTemplate` | `LibvirtCluster` template for ClusterClass |
| `LibvirtMachineTemplate` | `LibvirtMachine` template for `KubeadmControlPlane`, `MachineDeployment` and ClusterClass |

## LibvirtCluster

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: LibvirtCluster
metadata:
  name: libvirt-xd
spec:
  uri: qemu+tcp://libvirt.lab/system
  network: default
  basePool: capi-base-images
  domainPool: capi-vm-disks
  controlPlaneEndpoint:
    host: 192.168.122.10
    port: 6443
```

| Field | Required | Description |
|---|---|---|
| `uri` | yes | libvirt connection URI |
| `network` | yes | libvirt network the VMs are attached to |
| `basePool` | yes | Storage pool with the base images |
| `domainPool` | yes | Storage pool for VM disks and cloud-init ISOs |
| `controlPlaneEndpoint` | no | API server endpoint. Can be set here or on the `Cluster` |

## LibvirtMachine

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: LibvirtMachine
metadata:
  name: libvirt-xd-0
spec:
  vcpu: 2
  memoryMiB: 2048
  diskGiB: 20
  image: ubuntu-noble-base.qcow2
```

| Field | Required | Default | Minimum | Description |
|---|---|---|---|---|
| `image` | yes | | | Name of the base image volume in `basePool` |
| `vcpu` | no | `1` | `1` | Number of virtual CPUs |
| `memoryMiB` | no | `1024` | `256` | Memory in MiB |
| `diskGiB` | no | `20` | `1` | Root disk size in GiB |
| `providerID` | no | | | Set by the controller, see [how-it-works.md](how-it-works.md#names-and-identity) |

The spec cannot be changed after creation. `providerID` can be set once and then not changed.

## Templates

`LibvirtClusterTemplate` and `LibvirtMachineTemplate` use the same spec under `spec.template.spec`,
with the same validation. A `LibvirtMachineTemplate` cannot be changed either. To resize machines,
create a new template and point the `MachineDeployment` or `KubeadmControlPlane` to it.

## kubectl output

```sh
kubectl get libvirtclusters   # Cluster, Provisioned, URI, Endpoint, Age
kubectl get libvirtmachines   # Cluster, Provisioned, State, Address, Age
kubectl get libvirtmachines -o wide   # adds ProviderID
```

`State` is the reason of the `DomainRunning` condition, see
[how-it-works.md](how-it-works.md#conditions).
