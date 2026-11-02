# -*- mode: Python -*-
# Local development loop for cluster-api-provider-libvirt, see docs/development.md.
# It builds from the Dockerfile because CAPI's Tiltfile builds providers without cgo.

version_settings(constraint = ">=0.33.0")

CONTROLLER_IMAGE = "controller"
CONTROLLER_DEPLOYMENT = "cluster-api-provider-libvirt-controller-manager"
CAPI_NAMESPACE = "capi-system"
CAPI_DEPLOYMENT = "capi-controller-manager"
LABEL = "CAPLV"
CAPI_LABEL = "CAPI"

def capi_version_from_go_mod():
    """Returns the cluster-api version from go.mod, so the core matches the code."""
    for line in str(read_file("go.mod")).splitlines():
        fields = line.strip().split(" ")
        if len(fields) == 2 and fields[0] == "sigs.k8s.io/cluster-api":
            return fields[1]
    fail("sigs.k8s.io/cluster-api not found in go.mod")

settings = {
    # Extra kube contexts Tilt may deploy to; local ones like kind-* are always allowed.
    "allowed_contexts": [],
    # Install CAPI core, kubeadm bootstrap and kubeadm control plane with clusterctl.
    "install_capi": True,
    "capi_version": capi_version_from_go_mod(),
    # Variables for the "cluster" button, which applies templates/cluster-template.yaml.
    "cluster_name": "dev",
    "cluster_namespace": "default",
    "cluster_template_vars": {},
}
settings.update(read_yaml("tilt-settings.yaml", default = {}))

allow_k8s_contexts(settings.get("allowed_contexts"))

controller_deps = []

if settings.get("install_capi"):
    capi_version = settings.get("capi_version")
    local_resource(
        "capi",
        cmd = "kubectl get deployment -n {ns} {deploy} >/dev/null 2>&1 || clusterctl init --core cluster-api:{v} --bootstrap kubeadm:{v} --control-plane kubeadm:{v} --wait-providers".format(
            ns = CAPI_NAMESPACE,
            deploy = CAPI_DEPLOYMENT,
            v = capi_version,
        ),
        labels = [CAPI_LABEL],
    )
    controller_deps.append("capi")

docker_build(
    CONTROLLER_IMAGE,
    context = ".",
    dockerfile = "Dockerfile",
)

kustomize_bin = "./bin/kustomize" if os.path.exists("bin/kustomize") else None
k8s_yaml(kustomize("config/default", kustomize_bin = kustomize_bin))

k8s_resource(
    CONTROLLER_DEPLOYMENT,
    new_name = "caplv_controller",
    labels = [LABEL],
    resource_deps = controller_deps,
)

local_resource(
    "cluster",
    cmd = "clusterctl generate cluster {name} --target-namespace {ns} --from templates/cluster-template.yaml | kubectl apply -f -".format(
        name = settings.get("cluster_name"),
        ns = settings.get("cluster_namespace"),
    ),
    env = {k: str(v) for k, v in settings.get("cluster_template_vars").items()},
    auto_init = False,
    trigger_mode = TRIGGER_MODE_MANUAL,
    resource_deps = ["caplv_controller"],
    labels = [LABEL],
)
