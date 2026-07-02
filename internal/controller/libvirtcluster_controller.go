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
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"

	infrav1 "github.com/thebhdn/cluster-api-provider-libvirt/api/v1alpha1"
	"github.com/thebhdn/cluster-api-provider-libvirt/internal/libvirtclient"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	requeueTimeShort  = 30 * time.Second
	requeueTimeMedium = 3 * time.Minute
	requeueTimeLong   = 5 * time.Minute
)

// LibvirtClusterReconciler reconciles a LibvirtCluster object
type LibvirtClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// ClusterScope is a struct that contains the necessary data needed for a LibvirtCluster controller
type ClusterScope struct {
	Cluster        *clusterv1.Cluster
	Ctx            context.Context
	LibvirtCluster *infrav1.LibvirtCluster
	libvirtclient.InfraConfig
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=libvirtclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=libvirtclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=libvirtclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status;machinesets;machines;machines/status;machinepools;machinepools/status,verbs=get;list;watch;create;update;patch;delete

func (r *LibvirtClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, rerr error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling libvirt-cluster", "cluster-name", req.Name, "cluster-namespace", req.Namespace)

	libvirtCluster := &infrav1.LibvirtCluster{}

	err := r.Get(ctx, req.NamespacedName, libvirtCluster)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("libvirtCluster not found", "cluster-name", req.Name, "cluster-namespace", req.Namespace)

			return ctrl.Result{}, nil
		}
		logger.Error(err, "Error happened when getting libvirt-cluster",
			"cluster-name", req.Name,
			"cluster-namespace", req.Namespace)

		return ctrl.Result{}, err
	}

	clusterOwner, err := util.GetOwnerCluster(ctx, r.Client, libvirtCluster.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}

	helper, err := patch.NewHelper(libvirtCluster, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	defer func() {
		if patchErr := helper.Patch(ctx, libvirtCluster); patchErr != nil {
			logger.Error(patchErr, "unable to patch", "cluster", client.ObjectKeyFromObject(libvirtCluster).String())
			if rerr == nil {
				rerr = patchErr
			}
		}
	}()

	if clusterOwner == nil {
		logger.Info("Waiting for Cluster Controller to set OwnerRef on LibvirtCluster")
		return ctrl.Result{}, nil
	}

	scope := &ClusterScope{
		Cluster:        clusterOwner,
		LibvirtCluster: libvirtCluster,
		Ctx:            ctx,
		InfraConfig:    newInfraConfig(libvirtCluster),
	}

	return r.reconcileNormal(scope)
}

// SetupWithManager sets up the controller with the Manager.
func (r *LibvirtClusterReconciler) SetupWithManager(_ context.Context, mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.LibvirtCluster{}).
		Named("libvirtcluster").
		Complete(r)
}

func (r *LibvirtClusterReconciler) reconcileNormal(scope *ClusterScope) (ctrl.Result, error) {
	logger := log.FromContext(scope.Ctx)

	if !controllerutil.ContainsFinalizer(scope.LibvirtCluster, infrav1.LibvirtClusterFinalizer) {
		controllerutil.AddFinalizer(scope.LibvirtCluster, infrav1.LibvirtClusterFinalizer)
		return ctrl.Result{}, nil
	}

	conditions.Set(scope.LibvirtCluster, v1.Condition{
		Type:    infrav1.InfrastructureReadyCondition,
		Status:  v1.ConditionFalse,
		Reason:  infrav1.InfrastructureProvisioningInProgressReason,
		Message: "Infrastructure provisioning in progress",
	})

	// TODO: trigger check based on set conditions
	if err := ensureInfra(scope); err != nil {
		logger.Error(err, "could not verify libvirt infrastructure, requeuing....")

		conditions.Set(scope.LibvirtCluster, v1.Condition{
			Type:    infrav1.InfrastructureReadyCondition,
			Status:  v1.ConditionFalse,
			Reason:  infrav1.InfrastructureProvisioningFailedReason,
			Message: err.Error(),
		})
		// TODO: define sentinel errs
		return ctrl.Result{RequeueAfter: requeueTimeShort}, nil
	}

	conditions.Set(scope.LibvirtCluster, v1.Condition{
		Type:    infrav1.InfrastructureReadyCondition,
		Status:  v1.ConditionTrue,
		Reason:  infrav1.InfrastructureReadyCondition,
		Message: "All infrastructure components are ready",
	})

	scope.LibvirtCluster.Status.Ready = true
	scope.LibvirtCluster.Status.Initialization.Provisioned = true

	logger.Info("LibvirtCluster is provisioned", "cluster", scope.LibvirtCluster.Name)

	return ctrl.Result{}, nil
}

// func (r *LibvirtClusterReconciler) reconcileDelete(scope *ClusterScope) (ctrl.Result, error) {
// 	return ctrl.Result{}, nil
// }

// TODO: make cluster controller manage libvirt infra
func ensureInfra(s *ClusterScope) error {
	netActive, err := s.IsNetworkActive()
	if err != nil {
		return fmt.Errorf("error checking libvirt network %q: %w", s.Network, err)
	}
	if !netActive {
		return fmt.Errorf("network %q is not active", s.Network)
	}

	baseStoragePool, err := s.BasePoolExists()
	if err != nil {
		return fmt.Errorf("error checking base storage pool %q: %w", s.BasePool, err)
	}
	if !baseStoragePool {
		return fmt.Errorf("base storage pool %q is not active", s.BasePool)
	}

	vmStoragePool, err := s.VMStoragePoolExists()
	if err != nil {
		return fmt.Errorf("error checking vm storage pool %q: %w", s.BasePool, err)
	}
	if !vmStoragePool {
		return fmt.Errorf("base vm pool %q is not active", s.BasePool)
	}

	return nil
}

func newInfraConfig(libvirtCluster *infrav1.LibvirtCluster) libvirtclient.InfraConfig {
	return libvirtclient.InfraConfig{
		URI:        libvirtCluster.Spec.URI,
		BasePool:   libvirtCluster.Spec.BasePool,
		DomainPool: libvirtCluster.Spec.DomainPool,
		Network:    libvirtCluster.Spec.Network,
	}
}
