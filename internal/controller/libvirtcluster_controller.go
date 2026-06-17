/*
Copyright 2026.

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

func (r *LibvirtClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling libvirt-cluster", "cluster-name", req.Name, "cluster-namespace", req.Namespace)

	cluster := &infrav1.LibvirtCluster{}

	err := r.Get(ctx, req.NamespacedName, cluster)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("cluster not found", "cluster-name", req.Name, "cluster-namespace", req.Namespace)

			return ctrl.Result{}, nil
		}

		logger.Error(err, "Error happened when getting libvirt-cluster",
			"cluster-name", req.Name,
			"cluster-namespace", req.Namespace)

		return ctrl.Result{}, err
	}

	if !controllerutil.ContainsFinalizer(cluster, infrav1.LibvirtClusterFinalizer) {
		controllerutil.AddFinalizer(cluster, infrav1.LibvirtClusterFinalizer)
		return ctrl.Result{}, r.Update(ctx, cluster)
	}

	if !cluster.ObjectMeta.DeletionTimestamp.IsZero() {
		controllerutil.RemoveFinalizer(cluster, infrav1.LibvirtClusterFinalizer)
		return ctrl.Result{}, r.Update(ctx, cluster)
	}

	cluster.Status.Ready = true

	meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
		Type:               infrav1.ReadyCondition,
		Status:             metav1.ConditionTrue,
		Reason:             "LibvirtInfrastructureReady",
		Message:            "Libvirt cluster infrastructure is ready",
		ObservedGeneration: cluster.Generation,
	})

	return ctrl.Result{}, r.Status().Update(ctx, cluster)
}

// SetupWithManager sets up the controller with the Manager.
func (r *LibvirtClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.LibvirtCluster{}).
		Named("libvirtcluster").
		Complete(r)
}
