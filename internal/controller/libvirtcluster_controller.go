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
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"

	// "sigs.k8s.io/cluster-api/util"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	infrav1 "github.com/thebhdn/cluster-api-provider-libvirt/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LibvirtClusterReconciler reconciles a LibvirtCluster object
type LibvirtClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=libvirtclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=libvirtclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=libvirtclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch

func (r *LibvirtClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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

	// cluster, err := util.GetOwnerCluster(ctx, r.Client, libvirtCluster.ObjectMeta)
	// if err != nil {
	// 	return ctrl.Result{}, err
	// }

	// helper, err := patch.NewHelper(libvirtCluster, r.Client)
	// if err != nil {
	// 	return ctrl.Result{}, err
	// }

	// if cluster == nil {
	// 	logger.Info("Waiting for Cluster Controller to set OwnerRef on LibvirtCluster")
	// 	return ctrl.Result{}, nil
	// }

	if !controllerutil.ContainsFinalizer(libvirtCluster, infrav1.LibvirtClusterFinalizer) {
		controllerutil.AddFinalizer(libvirtCluster, infrav1.LibvirtClusterFinalizer)
		return ctrl.Result{}, r.Update(ctx, libvirtCluster)
	}

	if !libvirtCluster.ObjectMeta.DeletionTimestamp.IsZero() {
		controllerutil.RemoveFinalizer(libvirtCluster, infrav1.LibvirtClusterFinalizer)
		return ctrl.Result{}, r.Update(ctx, libvirtCluster)
	}

	libvirtCluster.Status.Ready = true

	meta.SetStatusCondition(&libvirtCluster.Status.Conditions, metav1.Condition{
		Type:               infrav1.ReadyCondition,
		Status:             metav1.ConditionTrue,
		Reason:             "LibvirtInfrastructureReady",
		Message:            "Libvirt cluster infrastructure is ready",
		ObservedGeneration: libvirtCluster.Generation,
	})

	// if err := helper.Patch(ctx, libvirtCluster); err != nil {
	// 	return ctrl.Result{}, errors.Wrapf(err, "couldn't patch libvirt cluster %q", libvirtCluster.Name)
	// }

	return ctrl.Result{}, r.Status().Update(ctx, libvirtCluster)
}

// SetupWithManager sets up the controller with the Manager.
func (r *LibvirtClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.LibvirtCluster{}).
		Named("libvirtcluster").
		Complete(r)
}
