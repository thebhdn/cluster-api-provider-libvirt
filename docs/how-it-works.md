# How it works

```
Management cluster                             libvirt host
┌────────────────────────────────┐             ┌──────────────────────────────────┐
│ CAPI core + bootstrap provider │             │ network     (DHCP, e.g. NAT)     │
│ cluster-api-provider-libvirt   │──libvirt──> │ basePool    (base images, qcow2) │
│   LibvirtCluster controller    │     API     │ domainPool  (VM disks, ISOs)     │
│   LibvirtMachine controller    │             │ domains     (one per machine)    │
└────────────────────────────────┘             └──────────────────────────────────┘
```

## LibvirtCluster

1. Waits until CAPI sets the owner `Cluster`, then adds a finalizer.
2. Checks that a control plane endpoint is set, either on `LibvirtCluster.spec.controlPlaneEndpoint`
   or on `Cluster.spec.controlPlaneEndpoint`. If not, it sets `ControlPlaneEndpointReady=False` and waits.
3. Reads the credentials from `spec.credentialsRef`, or else from the controller-wide Secret.
4. Connects to `spec.uri` and checks that `spec.network`, `spec.basePool` and `spec.domainPool`
   exist and are active. It does not create or start them.
5. Sets `status.initialization.provisioned=true` when everything is ready.

On delete, it only removes the finalizer. Nothing on the host is changed.

## LibvirtMachine

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
   `status.addresses`. It checks the domain again every minute, so changes on the host are seen.
5. If the domain was created before but is now gone from the host, it does not recreate it. It
   sets `DomainRunning=False` with reason `DomainNotFound`. Delete the `Machine` (or let a
   `MachineHealthCheck` do it) to get a new one.

On delete, it removes the domain, the root disk, the cloud-init ISO and the DHCP reservation.

## Names and identity

| Item | Value |
|---|---|
| Domain name | `<namespace>-<name>-<hash>`, for example `default-libvirt-xd-0-3fa91c`. Unique on the host and a valid DNS label |
| Guest hostname | The `LibvirtMachine` name |
| Domain UUID | The `LibvirtMachine` UID |
| Provider ID | `libvirt://<LibvirtMachine UID>` |
| cloud-init `instance-id` | The domain UUID |
| Volumes | `<domain name>.qcow2` and `cloudinit-<domain name>.iso` in `domainPool` |
| MAC address | Derived from the domain name, with the `52:54:00` prefix |

## Conditions

| Resource | Condition | Meaning |
|---|---|---|
| `LibvirtCluster` | `ControlPlaneEndpointReady` | A control plane endpoint is set |
| `LibvirtCluster` | `InfrastructureReady` | Network and pools exist and are active (reasons: `CredentialsUnavailable`, `InfrastructureMissing`, `InfrastructureInactive`, `InfrastructureProvisioningFailed`) |
| `LibvirtMachine` | `InfrastructureReady` | The cluster infrastructure is provisioned |
| `LibvirtMachine` | `DomainProvisioningReady` | The domain is being created or creation failed |
| `LibvirtMachine` | `MachineCreated` | The domain was created |
| `LibvirtMachine` | `DomainRunning` | The domain is running (reasons: `DomainNotRunning`, `DomainNotFound`) |
