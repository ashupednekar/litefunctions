/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http:
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// +kubebuilder:rbac:groups=apps.ashupednekar.github.io,resources=functions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
package controller

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	apiv1 "github.com/ashupednekar/litefunctions/operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
)

type FunctionReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *FunctionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var function apiv1.Function
	if err := r.Get(ctx, req.NamespacedName, &function); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	deploymentName := GetDeploymentName(&function)
	serviceName := GetServiceName(&function)

	if !function.Spec.IsActive {
		var existing appsv1.Deployment
		err := r.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: function.Namespace}, &existing)

		if err == nil {
			if err := r.Delete(ctx, &existing); err != nil {
				log.Error(err, "Failed to delete deployment", "deployment", deploymentName)
				return ctrl.Result{}, err
			}
			log.Info("Deleted deployment for inactive function", "deployment", deploymentName)
		} else if !apierrs.IsNotFound(err) {
			log.Error(err, "Failed to get deployment", "deployment", deploymentName)
			return ctrl.Result{}, err
		}

		var existingSvc corev1.Service
		svcErr := r.Get(ctx, types.NamespacedName{Name: serviceName, Namespace: function.Namespace}, &existingSvc)
		if svcErr == nil {
			if err := r.Delete(ctx, &existingSvc); err != nil {
				log.Error(err, "Failed to delete service", "service", serviceName)
				return ctrl.Result{}, err
			}
			log.Info("Deleted service for inactive function", "service", serviceName)
		} else if !apierrs.IsNotFound(svcErr) {
			log.Error(svcErr, "Failed to get service", "service", serviceName)
			return ctrl.Result{}, svcErr
		}

		return ctrl.Result{}, nil
	}

	deploy := NewDeployment(&function)
	svc := NewService(&function)

	if err := controllerutil.SetControllerReference(&function, deploy, r.Scheme); err != nil {
		log.Error(err, "Failed to set controller reference")
		return ctrl.Result{}, err
	}
	if err := controllerutil.SetControllerReference(&function, svc, r.Scheme); err != nil {
		log.Error(err, "Failed to set controller reference for service")
		return ctrl.Result{}, err
	}

	var existing appsv1.Deployment
	err := r.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: function.Namespace}, &existing)

	if err != nil && apierrs.IsNotFound(err) {
		if err := r.Create(ctx, deploy); err != nil {
			log.Error(err, "Failed to create deployment", "deployment", deploymentName)
			return ctrl.Result{}, err
		}
		log.Info("Created new deployment for function", "deployment", deploymentName)
	} else if err == nil {
		deploy.ResourceVersion = existing.ResourceVersion
		if err := r.Update(ctx, deploy); err != nil {
			log.Error(err, "Failed to update deployment", "deployment", deploymentName)
			return ctrl.Result{}, err
		}
		log.Info("Updated existing deployment for function", "deployment", deploymentName)
	} else {
		log.Error(err, "Failed to get deployment", "deployment", deploymentName)
		return ctrl.Result{}, err
	}

	var existingSvc corev1.Service
	svcErr := r.Get(ctx, types.NamespacedName{Name: serviceName, Namespace: function.Namespace}, &existingSvc)
	if svcErr != nil && apierrs.IsNotFound(svcErr) {
		if err := r.Create(ctx, svc); err != nil {
			log.Error(err, "Failed to create service", "service", serviceName)
			return ctrl.Result{}, err
		}
		log.Info("Created new service for function", "service", serviceName)
	} else if svcErr == nil {
		svc.ResourceVersion = existingSvc.ResourceVersion
		svc.Spec.ClusterIP = existingSvc.Spec.ClusterIP
		svc.Spec.ClusterIPs = existingSvc.Spec.ClusterIPs
		if err := r.Update(ctx, svc); err != nil {
			log.Error(err, "Failed to update service", "service", serviceName)
			return ctrl.Result{}, err
		}
		log.Info("Updated existing service for function", "service", serviceName)
	} else {
		log.Error(svcErr, "Failed to get service", "service", serviceName)
		return ctrl.Result{}, svcErr
	}

	now := time.Now()
	deprovisionTime := now.Add(Cfg.KeepWarmDuration)
	function.Spec.DeProvisionTime = deprovisionTime.Format(time.RFC3339)

	if err := r.Update(ctx, &function); err != nil {
		log.Error(err, "Failed to update function deprovision time")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *FunctionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1.Function{}).
		Named("function").
		Complete(r)
}
