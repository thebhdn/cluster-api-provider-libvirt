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
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	infrav1 "github.com/thebhdn/cluster-api-provider-libvirt/api/v1alpha1"
)

// LibvirtMachineReconciler reconciles a LibvirtMachine object
type LibvirtMachineReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=libvirtmachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=libvirtmachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=libvirtmachines/finalizers,verbs=update

func (r *LibvirtMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling libvirt machine")

	machine := &infrav1.LibvirtMachine{}

	err := r.Get(ctx, req.NamespacedName, machine)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("harvestermachine not found")

			return ctrl.Result{}, nil
		}

		logger.Error(err, "Error happened when getting harvestermachine")

		return ctrl.Result{}, err
	}

	if !controllerutil.ContainsFinalizer(machine, infrav1.LibvirtMachineFinalizer) {
		controllerutil.AddFinalizer(machine, infrav1.LibvirtMachineFinalizer)
		return ctrl.Result{}, r.Update(ctx, machine)
	}

	if !machine.ObjectMeta.DeletionTimestamp.IsZero() {
		controllerutil.RemoveFinalizer(machine, infrav1.LibvirtMachineFinalizer)
		return ctrl.Result{}, r.Update(ctx, machine)
	}

	if machine.Spec.ProviderID == nil {
		providerID := fmt.Sprintf("libvirt://%s/%s", machine.Namespace, machine.Name)
		machine.Spec.ProviderID = &providerID
		if err := r.Update(ctx, machine); err != nil {
			return ctrl.Result{}, err
		}
	}

	machine.Status.Ready = true

	meta.SetStatusCondition(&machine.Status.Conditions, metav1.Condition{
		Type:               infrav1.ReadyCondition,
		Status:             metav1.ConditionTrue,
		Reason:             "LibvirtMachineReady",
		Message:            "Libvirt machine infrastructure is ready",
		ObservedGeneration: machine.Generation,
	})

	return ctrl.Result{}, r.Status().Update(ctx, machine)
}

// SetupWithManager sets up the controller with the Manager.
func (r *LibvirtMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.LibvirtMachine{}).
		Named("libvirtmachine").
		Complete(r)
}
