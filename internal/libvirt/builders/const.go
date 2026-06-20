package builders

const (
	DefaultDomainType = "kvm"
	DefaultMemoryMiB  = uint(1024)
	DefaultMemoryUnit = "MiB"
	DefaultVCPU       = uint(1)

	DefaultOSType = "hvm"
	DefaultOSArch = "x86_64"

	DefaultDiskDriverName = "qemu"
	DefaultDiskFormat     = "qcow2"
	DefaultDiskBus        = "virtio"

	DefaultNetworkName  = "default"
	DefaultNetworkModel = "virtio"

	DefaultCloudInitBus    = "sata"
	DefaultCloudInitTarget = "sda"

	DefaultBootDevHD = "hd"
)

const (
	DefaultVolumeCapacityGiB  = uint64(20)
	DefaultVolumeCapacityUnit = "G"
	DefaultVolumeFormat       = "qcow2"
)
