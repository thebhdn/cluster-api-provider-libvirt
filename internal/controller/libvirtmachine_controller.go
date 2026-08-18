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
	"fmt"
	"time"

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

// LibvirtMachineReconciler reconciles a LibvirtMachine object
type LibvirtMachineReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	Provider provider
}

type MachineScope struct {
	Cluster          *clusterv1.Cluster
	Machine          *clusterv1.Machine
	Ctx              context.Context
	LibvirtCluster   *infrav1.LibvirtCluster
	LibvirtMachine   *infrav1.LibvirtMachine
	MachineConfig    libvirtclient.MachineConfig
	ReconcilerClient client.Client
}

const (
	running = libvirtclient.DomainStateRunning
	stopped = libvirtclient.DomainStateStopped
	unknown = libvirtclient.DomainStateUnknown
	missing = libvirtclient.DomainStateNotFound
)

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
			logger.Info("LibvirtMachine not found")
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
			logger.Error(patchErr, "Unable to patch", "machine", client.ObjectKeyFromObject(libvirtMachine).String())
			if rerr == nil {
				rerr = patchErr
			}
		}
	}()

	ownerMachine, err := util.GetOwnerMachine(ctx, r.Client, libvirtMachine.ObjectMeta)
	if err != nil {
		logger.Error(err, "Unable to get owner machine")
		return ctrl.Result{}, err
	}

	if ownerMachine == nil {
		logger.Info("Waiting for machine controller to set OwnerRef on LibvirtMachine")
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
		logger.Error(err, "Unable to find corresponding libvirtCluster to libvirtMachine")
		return ctrl.Result{}, err
	}

	scope := &MachineScope{
		Cluster:          ownerCluster,
		Machine:          ownerMachine,
		LibvirtCluster:   libvirtCluster,
		LibvirtMachine:   libvirtMachine,
		MachineConfig:    newMachineConfig(libvirtMachine, libvirtCluster),
		Ctx:              ctx,
		ReconcilerClient: r.Client,
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

	cfg := scope.MachineConfig

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

	state, err := r.Provider.GetMachineState(cfg)
	if err != nil {
		logger.Error(err, "Unable to get domain state")
		return ctrl.Result{}, err
	}

	switch state {
	case missing:
		logger.Info("domain doesn't exist, creating....", "domain", cfg.DomainName)

		scope.LibvirtMachine.Status.Ready = false
		scope.LibvirtMachine.Status.Initialization.Provisioned = false

		conditions.Set(scope.LibvirtMachine, metav1.Condition{
			Type:    infrav1.DomainProvisioningReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.DomainProvisioningInProgressReason,
			Message: "Domain provisioning in progress",
		})

		if scope.Machine.Spec.Bootstrap.DataSecretName == nil {
			logger.Info("Waiting for Machine's Userdata to be set ... ")

			scope.LibvirtMachine.Status.Ready = false

			return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
		}

		cloudInitUserData, err := getCloudInitData(scope)
		if err != nil {
			err = fmt.Errorf("error during getting cloud init user data: %w", err)

			return ctrl.Result{}, err
		}

		scope.MachineConfig.UserData = cloudInitUserData

		info, err := r.Provider.CreateMachine(cfg)
		if err != nil {
			logger.Error(err, "Unable to create domain", "domain", cfg.DomainName)

			conditions.Set(scope.LibvirtMachine, metav1.Condition{
				Type:    infrav1.DomainProvisioningReadyCondition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav1.DomainProvisioningFailedReason,
				Message: fmt.Sprintf("Failed to create Domain: %v", err),
			})

			return ctrl.Result{}, err
		}

		providerID := "libvirt://" + info.ID
		scope.LibvirtMachine.Spec.ProviderID = &providerID

		conditions.Set(scope.LibvirtMachine, metav1.Condition{
			Type:   infrav1.MachineCreatedCondition,
			Status: metav1.ConditionTrue,
			Reason: infrav1.MachineCreatedCondition,
		})

		scope.LibvirtMachine.Status.Ready = true
		scope.LibvirtMachine.Status.Initialization.Provisioned = true

		return ctrl.Result{}, nil
	case stopped:
		logger.Info("Domain stopped, starting....", "domain", cfg.DomainName)

		scope.LibvirtMachine.Status.Ready = false

		if err := r.Provider.StartMachine(cfg); err != nil {
			logger.Error(err, "Unable to start domain", "domain", cfg.DomainName)

			conditions.Set(scope.LibvirtMachine, metav1.Condition{
				Type:    infrav1.DomainRunningCondition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav1.DomainNotRunningReason,
				Message: fmt.Sprintf("Failed start Domain: %v", err),
			})

			return ctrl.Result{}, err
		}

		scope.LibvirtMachine.Status.Ready = true

		return ctrl.Result{}, nil
	case running:
		logger.Info("Domain is running", "domain", cfg.DomainName)

		conditions.Set(scope.LibvirtMachine, metav1.Condition{
			Type:    infrav1.DomainRunningCondition,
			Status:  metav1.ConditionTrue,
			Reason:  infrav1.DomainRunningCondition,
			Message: "Domain is running",
		})

		scope.LibvirtMachine.Status.Ready = true
		scope.LibvirtMachine.Status.Initialization.Provisioned = true

		return ctrl.Result{}, nil
	case unknown:
		logger.Info("Domain state is unknown, requeuing", "domain", cfg.DomainName)

		scope.LibvirtMachine.Status.Ready = false
		scope.LibvirtMachine.Status.Initialization.Provisioned = false

		return ctrl.Result{RequeueAfter: requeueTimeShort}, nil
	}

	return ctrl.Result{}, nil
}

func (r *LibvirtMachineReconciler) reconcileDelete(scope *MachineScope) (ctrl.Result, error) {
	logger := log.FromContext(scope.Ctx)

	cfg := scope.MachineConfig

	logger.Info("Deleting domain", "domain", cfg.DomainName)

	err := r.Provider.DeleteMachine(cfg)
	if err != nil {
		logger.Error(err, "Unable to delete domain", "domain", cfg)
		return ctrl.Result{}, err
	}

	if ok := controllerutil.RemoveFinalizer(scope.LibvirtMachine, infrav1.LibvirtMachineFinalizer); !ok {
		return ctrl.Result{}, fmt.Errorf("unable to remove finalizer %s from LibvirtMachine  %s/%s",
			infrav1.LibvirtMachineFinalizer,
			scope.Machine.Namespace,
			scope.Machine.Name)
	}

	return ctrl.Result{}, nil
}

func newMachineConfig(libvirtMachine *infrav1.LibvirtMachine, libvirtCluster *infrav1.LibvirtCluster) libvirtclient.MachineConfig {
	return libvirtclient.MachineConfig{
		InfraConfig: libvirtclient.InfraConfig{
			URI:        libvirtCluster.Spec.URI,
			BasePool:   libvirtCluster.Spec.BasePool,
			DomainPool: libvirtCluster.Spec.DomainPool,
			Network:    libvirtCluster.Spec.Network,
		},
		DomainName: libvirtMachine.Name,
		BaseImage:  libvirtMachine.Spec.Image,
		MemoryMiB:  uint(libvirtMachine.Spec.MemoryMiB),
		VCPU:       uint(libvirtMachine.Spec.VCPU),
		DiskSize:   uint64(libvirtMachine.Spec.DiskGiB),
	}
}

func getCloudInitData(scope *MachineScope) ([]byte, error) {
	dataSecretNamespacedName := types.NamespacedName{
		Namespace: scope.Machine.Namespace,
		Name:      *scope.Machine.Spec.Bootstrap.DataSecretName,
	}

	dataSecret := &corev1.Secret{}

	err := scope.ReconcilerClient.Get(scope.Ctx, dataSecretNamespacedName, dataSecret)
	if err != nil {
		return nil, err
	}

	userData, ok := dataSecret.Data["value"]
	if !ok {
		return nil, fmt.Errorf("no userData key found in secret %s", dataSecretNamespacedName)
	}

	return userData, nil
}
