/*
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
*/

package controller

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"

	infrav1 "github.com/thebhdn/cluster-api-provider-libvirt/api/v1alpha1"
	"github.com/thebhdn/cluster-api-provider-libvirt/internal/libvirtclient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

// LibvirtMachineReconciler reconciles a LibvirtMachine object
type LibvirtMachineReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

type MachineScope struct {
	Cluster        *clusterv1.Cluster
	Machine        *clusterv1.Machine
	Ctx            context.Context
	LibvirtCluster *infrav1.LibvirtCluster
	LibvirtMachine *infrav1.LibvirtMachine
	libvirtclient.MachineConfig
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=libvirtmachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=libvirtmachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=libvirtmachines/finalizers,verbs=update
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch

func (r *LibvirtMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, rerr error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling libvirt machine")

	libvirtMachine := &infrav1.LibvirtMachine{}

	err := r.Get(ctx, req.NamespacedName, libvirtMachine)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("libvirtMachine not found")

			return ctrl.Result{}, nil
		}
		logger.Error(err, "Error happened when getting libvirtMachine")
		return ctrl.Result{}, err
	}

	// Initialize the patch helper
	helper, err := patch.NewHelper(libvirtMachine, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	defer func() {
		if patchErr := helper.Patch(ctx, libvirtMachine); patchErr != nil {
			logger.Error(patchErr, "unable to patch", "machine", client.ObjectKeyFromObject(libvirtMachine).String())
			if rerr == nil {
				rerr = patchErr
			}
		}
	}()

	ownerMachine, err := util.GetOwnerMachine(ctx, r.Client, libvirtMachine.ObjectMeta)
	if err != nil {
		logger.Error(err, "unable to get owner machine")
		return ctrl.Result{}, err
	}

	if ownerMachine == nil {
		logger.Info("waiting for machine controller to set OwnerRef on LibvirtMachine")
		return ctrl.Result{RequeueAfter: requeueTimeShort}, nil
	}

	ownerCluster, err := util.GetClusterFromMetadata(ctx, r.Client, ownerMachine.ObjectMeta)
	if err != nil {
		logger.Info("LibvirtMachine owner machine is missing cluster label or cluster does not exist")
		return ctrl.Result{}, err
	}

	if ownerCluster == nil {
		logger.Info("Please link this machine with a cluster using the label " + clusterv1.ClusterNameLabel + ": <name of cluster>")
		return ctrl.Result{}, nil
	}

	logger = logger.WithValues("machine", ownerMachine.Namespace+"/"+ownerMachine.Name, "cluster", ownerCluster.Namespace+"/"+ownerCluster.Name)

	libvirtCluster := &infrav1.LibvirtCluster{}

	libvirtClusterKey := types.NamespacedName{
		Namespace: ownerCluster.Namespace,
		Name:      ownerCluster.Spec.InfrastructureRef.Name,
	}

	err = r.Get(ctx, libvirtClusterKey, libvirtCluster)
	if err != nil {
		logger.Error(err, "unable to find corresponding libvirtCluster to libvirtMachine")
		return ctrl.Result{}, err
	}

	scope := &MachineScope{
		Cluster:        ownerCluster,
		Machine:        ownerMachine,
		LibvirtCluster: libvirtCluster,
		LibvirtMachine: libvirtMachine,
		MachineConfig:  newMachineConfig(libvirtMachine),
		Ctx:            ctx,
	}

	if !libvirtMachine.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(scope)
	}

	return r.reconcileNormal(scope)
}

// SetupWithManager sets up the controller with the Manager.
func (r *LibvirtMachineReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	clusterToLibvirtMachine, err := util.ClusterToTypedObjectsMapper(mgr.GetClient(), &infrav1.LibvirtMachineList{}, mgr.GetScheme())
	if err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.LibvirtMachine{}).
		Watches(
			&clusterv1.Machine{},
			handler.EnqueueRequestsFromMapFunc(util.MachineToInfrastructureMapFunc(infrav1.GroupVersion.WithKind("LibvirtMachine"))),
			builder.WithPredicates(predicates.ResourceNotPaused(mgr.GetScheme(), ctrl.LoggerFrom(ctx))),
		).
		Watches(
			&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(clusterToLibvirtMachine),
			builder.WithPredicates(predicates.ClusterUnpaused(mgr.GetScheme(), ctrl.LoggerFrom(ctx))),
		).
		Named("libvirtmachine").
		Complete(r)
}

func (r *LibvirtMachineReconciler) reconcileNormal(scope *MachineScope) (ctrl.Result, error) {
	logger := log.FromContext(scope.Ctx)

	if annotations.IsPaused(scope.Cluster, scope.LibvirtMachine) {
		logger.Info("Reconciliation is paused for this object")

		scope.LibvirtMachine.Status.Ready = false
		scope.LibvirtMachine.Status.Initialization.Provisioned = false

		return ctrl.Result{}, nil
	}

	// Add finalizer first if not exist to avoid the race condition between init and delete
	if !controllerutil.ContainsFinalizer(scope.LibvirtMachine, infrav1.LibvirtMachineFinalizer) && scope.LibvirtMachine.DeletionTimestamp.IsZero() {
		controllerutil.AddFinalizer(scope.LibvirtMachine, infrav1.LibvirtMachineFinalizer)

		scope.LibvirtMachine.Status.Ready = false
		scope.LibvirtMachine.Status.Initialization.Provisioned = false

		return ctrl.Result{}, nil
	}

	infraProvisioned := scope.Cluster.Status.Initialization.InfrastructureProvisioned
	if infraProvisioned == nil || !*infraProvisioned {
		logger.Info("Waiting for Infrastructure to be ready...")

		scope.LibvirtMachine.Status.Ready = false
		scope.LibvirtMachine.Status.Initialization.Provisioned = false

		conditions.Set(scope.LibvirtMachine, metav1.Condition{
			Type:    infrav1.InfrastructureReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.InfrastructureProvisioningInProgressReason,
			Message: "Waiting for cluster infrastructure to be ready",
		})

		return ctrl.Result{RequeueAfter: requeueTimeShort}, nil
	}

	// Set InfrastructureReady condition when cluster infrastructure is ready
	conditions.Set(scope.LibvirtMachine, metav1.Condition{
		Type:    infrav1.InfrastructureReadyCondition,
		Status:  metav1.ConditionTrue,
		Reason:  infrav1.InfrastructureReadyReason,
		Message: "Cluster infrastructure is ready",
	})

	state, err := scope.GetDomainState()
	if err != nil {
		logger.Error(err, "unable to get domain state")
		return ctrl.Result{}, err
	}

	switch state {
	case libvirtclient.DomainStateNotFound:
		logger.Info("domain doesn't exist, creating....", "domain", scope.DomainName)
		return ctrl.Result{}, nil
	case libvirtclient.DomainStateStopped:
		logger.Info("domain stopped, starting....", "domain", scope.DomainName)
		return ctrl.Result{}, nil
	case libvirtclient.DomainStateRunning:
		logger.Info("domain is running", "domain", scope.DomainName)
		return ctrl.Result{}, nil
	case libvirtclient.DomainStateUnknown:
		logger.Info("domain state is uknown, requeuing", "domain", scope.DomainName)
		return ctrl.Result{RequeueAfter: requeueTimeShort}, nil
	}

	return ctrl.Result{}, nil
}

func (r *LibvirtMachineReconciler) reconcileDelete(scope *MachineScope) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

func newMachineConfig(libvirtMachine *infrav1.LibvirtMachine) libvirtclient.MachineConfig {
	return libvirtclient.MachineConfig{
		BaseImage: libvirtMachine.Spec.Image,
		MemoryMiB: uint(libvirtMachine.Spec.MemoryMiB),
		VCPU:      uint(libvirtMachine.Spec.VCPU),
		DiskSize:  uint64(libvirtMachine.Spec.DiskGiB),
	}
}
