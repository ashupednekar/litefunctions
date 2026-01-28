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

// +kubebuilder:rbac:groups=apps.ashupednekar.github.io,resources=functions,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
package controller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	apiv1 "github.com/ashupednekar/litefunctions/operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type FunctionReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// TODO: create pull secret
func (r *FunctionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var function apiv1.Function
	if err := r.Get(ctx, req.NamespacedName, &function); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	labels := map[string]string{
		"app":  "runtime",
		"lang": function.Spec.Language,
		"project": function.Spec.Project,
		"function": function.Spec.Name,
	}

	deploymentName := fmt.Sprintf("litefunctions-runtime-%s-%s-%s", function.Spec.Language, function.Spec.Project, function.Name)

	var image string
	switch function.Spec.Language {
	case "python":
		image = "ashupednekar535/litefunctions-runtime-py:latest"
	case "js":
		image = "ashupednekar535/litefunctions-runtime-js:latest"
	case "lua":
		image = "ashupednekar535/litefunctions-runtime-lua:latest"
	default:
		image = fmt.Sprintf("%s/%s/runtime-%s-%s-%s:latest", Cfg.Registry, Cfg.VcsUser, function.Spec.Language, function.Spec.Project, function.Name)
	}

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: function.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32(1),
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					ImagePullSecrets: []corev1.LocalObjectReference{
						{Name: Cfg.PullSecret},
					},
					Containers: []corev1.Container{
						{
							Name:            deploymentName,
							Image:           image,
							ImagePullPolicy: corev1.PullAlways,
							Env: []corev1.EnvVar{ // TODO: accept user provided values/secrets
								{
									Name: "DATABASE_URL",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: Cfg.DbSecretName,
											},
											Key: Cfg.DbSecretKey,
										},
									},
								},
								{
									Name:  "REDIS_URL",
									Value: Cfg.RedisUrl,
								},
								{
									Name:  "NATS_URL", //TODO: multiple brokers
									Value: Cfg.NatsUrl, 
								},
							},
						},
					},
				},
			},
		},
	}

	// Set controller reference
	if err := controllerutil.SetControllerReference(&function, deploy, r.Scheme); err != nil {
		log.Error(err, "Failed to set controller reference")
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

	return ctrl.Result{}, nil
}

func (r *FunctionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1.Function{}).
		Named("function").
		Complete(r)
}
