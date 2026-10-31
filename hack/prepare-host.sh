#!/usr/bin/env bash
# Copyright 2026 Bohdan Leshchenko.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Prepares a libvirt host for cluster-api-provider-libvirt. Safe to run again.
# It uses the same variables as templates/cluster-template.yaml.

set -euo pipefail

export LC_ALL=C

LIBVIRT_URI="${LIBVIRT_URI:-qemu:///system}"
LIBVIRT_NETWORK="${LIBVIRT_NETWORK:-default}"
LIBVIRT_BASE_POOL="${LIBVIRT_BASE_POOL:-capi-base-images}"
LIBVIRT_DOMAIN_POOL="${LIBVIRT_DOMAIN_POOL:-capi-vm-disks}"
LIBVIRT_POOL_ROOT="${LIBVIRT_POOL_ROOT:-/var/lib/libvirt/capi}"
LIBVIRT_IMAGE="${LIBVIRT_IMAGE:-}"
IMAGE_FILE=""
FORCE=false
SKIP_NETWORK=false

QCOW2_MAGIC="QFI"

usage() {
	cat <<EOF
Usage: $(basename "$0") [options]

Checks the libvirt network, creates the storage pools and uploads a base image.
Running it again changes nothing that is already in place.

Options (each one can also be set with the variable in brackets):
  --uri URI              libvirt connection URI [LIBVIRT_URI] (default: ${LIBVIRT_URI})
  --network NAME         libvirt network with DHCP [LIBVIRT_NETWORK] (default: ${LIBVIRT_NETWORK})
  --base-pool NAME       pool for base images [LIBVIRT_BASE_POOL] (default: ${LIBVIRT_BASE_POOL})
  --domain-pool NAME     pool for VM disks and cloud-init ISOs [LIBVIRT_DOMAIN_POOL] (default: ${LIBVIRT_DOMAIN_POOL})
  --pool-root DIR        directory on the host for new pools [LIBVIRT_POOL_ROOT] (default: ${LIBVIRT_POOL_ROOT})
  --image FILE           local qcow2 file to upload into the base pool
  --image-name NAME      volume name for --image [LIBVIRT_IMAGE] (default: the file name)
  --force                replace the image volume if it already exists
  --skip-network         do not check the network
  -h, --help             show this help

Example:
  $(basename "$0") --uri qemu+tls://libvirt.lab/system --image output/ubuntu-2404-kube-v1.34.1
EOF
}

log() {
	echo "==> $*"
}

fail() {
	echo "error: $*" >&2
	exit 1
}

virsh_cmd() {
	virsh -q -c "${LIBVIRT_URI}" "$@"
}

# info_field prints one "Key: value" field from virsh net-info or pool-info output.
info_field() {
	local field="$1"
	awk -F': *' -v field="${field}" '$1 == field { print $2 }'
}

parse_args() {
	while [[ $# -gt 0 ]]; do
		case "$1" in
		--uri) LIBVIRT_URI="$2"; shift 2 ;;
		--network) LIBVIRT_NETWORK="$2"; shift 2 ;;
		--base-pool) LIBVIRT_BASE_POOL="$2"; shift 2 ;;
		--domain-pool) LIBVIRT_DOMAIN_POOL="$2"; shift 2 ;;
		--pool-root) LIBVIRT_POOL_ROOT="$2"; shift 2 ;;
		--image) IMAGE_FILE="$2"; shift 2 ;;
		--image-name) LIBVIRT_IMAGE="$2"; shift 2 ;;
		--force) FORCE=true; shift ;;
		--skip-network) SKIP_NETWORK=true; shift ;;
		-h | --help) usage; exit 0 ;;
		*) usage >&2; fail "unknown option: $1" ;;
		esac
	done
}

check_connection() {
	command -v virsh >/dev/null || fail "virsh not found, install the libvirt client tools"
	virsh_cmd uri >/dev/null || fail "cannot connect to ${LIBVIRT_URI}"
	log "Connected to ${LIBVIRT_URI}"
}

ensure_network() {
	local info
	info="$(virsh_cmd net-info "${LIBVIRT_NETWORK}" 2>/dev/null)" ||
		fail "network ${LIBVIRT_NETWORK} not found, create a network with DHCP first (for example the default NAT network)"

	virsh_cmd net-dumpxml "${LIBVIRT_NETWORK}" | grep -q "<dhcp" ||
		fail "network ${LIBVIRT_NETWORK} has no DHCP, the provider needs it to give VMs an address"

	if [[ "$(info_field Active <<<"${info}")" != "yes" ]]; then
		log "Starting network ${LIBVIRT_NETWORK}"
		virsh_cmd net-start "${LIBVIRT_NETWORK}" >/dev/null
	fi

	if [[ "$(info_field Autostart <<<"${info}")" != "yes" ]]; then
		log "Turning on autostart for network ${LIBVIRT_NETWORK}"
		virsh_cmd net-autostart "${LIBVIRT_NETWORK}" >/dev/null
	fi

	log "Network ${LIBVIRT_NETWORK} is active with DHCP"
}

ensure_pool() {
	local pool="$1"
	local target="${LIBVIRT_POOL_ROOT}/${pool}"
	local info

	if ! info="$(virsh_cmd pool-info "${pool}" 2>/dev/null)"; then
		log "Creating pool ${pool} in ${target}"
		virsh_cmd pool-define-as "${pool}" dir --target "${target}" >/dev/null
		virsh_cmd pool-build "${pool}" >/dev/null
		info="$(virsh_cmd pool-info "${pool}")"
	fi

	if [[ "$(info_field State <<<"${info}")" != "running" ]]; then
		log "Starting pool ${pool}"
		virsh_cmd pool-start "${pool}" >/dev/null
	fi

	if [[ "$(info_field Autostart <<<"${info}")" != "yes" ]]; then
		log "Turning on autostart for pool ${pool}"
		virsh_cmd pool-autostart "${pool}" >/dev/null
	fi

	log "Pool ${pool} is active"
}

check_image_file() {
	[[ -f "${IMAGE_FILE}" ]] || fail "image file ${IMAGE_FILE} not found"
	[[ "$(head -c "${#QCOW2_MAGIC}" "${IMAGE_FILE}")" == "${QCOW2_MAGIC}" ]] ||
		fail "image file ${IMAGE_FILE} is not a qcow2 image"
}

upload_image() {
	local name="${LIBVIRT_IMAGE:-$(basename "${IMAGE_FILE}")}"

	virsh_cmd pool-refresh "${LIBVIRT_BASE_POOL}" >/dev/null
	if virsh_cmd vol-info --pool "${LIBVIRT_BASE_POOL}" "${name}" >/dev/null 2>&1; then
		if [[ "${FORCE}" != "true" ]]; then
			log "Image ${name} already exists in pool ${LIBVIRT_BASE_POOL}, use --force to replace it"
			print_image_name "${name}"
			return
		fi
		log "Deleting image ${name} from pool ${LIBVIRT_BASE_POOL}"
		virsh_cmd vol-delete --pool "${LIBVIRT_BASE_POOL}" "${name}" >/dev/null
	fi

	local size
	size="$(wc -c <"${IMAGE_FILE}")"

	log "Uploading ${IMAGE_FILE} to pool ${LIBVIRT_BASE_POOL} as ${name} ($((size / 1024 / 1024)) MiB)"
	virsh_cmd vol-create-as "${LIBVIRT_BASE_POOL}" "${name}" "${size}" --format qcow2 >/dev/null
	if ! virsh_cmd vol-upload --pool "${LIBVIRT_BASE_POOL}" "${name}" "${IMAGE_FILE}"; then
		virsh_cmd vol-delete --pool "${LIBVIRT_BASE_POOL}" "${name}" >/dev/null 2>&1 || true
		fail "upload of ${IMAGE_FILE} failed, the partial volume was deleted"
	fi
	virsh_cmd pool-refresh "${LIBVIRT_BASE_POOL}" >/dev/null

	print_image_name "${name}"
}

print_image_name() {
	log "Use LIBVIRT_IMAGE=$1 in the cluster template"
}

main() {
	parse_args "$@"
	if [[ -n "${IMAGE_FILE}" ]]; then
		check_image_file
	fi
	check_connection

	if [[ "${SKIP_NETWORK}" != "true" ]]; then
		ensure_network
	fi

	ensure_pool "${LIBVIRT_BASE_POOL}"
	if [[ "${LIBVIRT_DOMAIN_POOL}" != "${LIBVIRT_BASE_POOL}" ]]; then
		ensure_pool "${LIBVIRT_DOMAIN_POOL}"
	fi

	if [[ -n "${IMAGE_FILE}" ]]; then
		upload_image
	fi

	log "Host is ready"
}

main "$@"
