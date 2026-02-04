package controller

import (
	"fmt"

	"k8s.io/utils/pointer"

	apiv1 "github.com/ashupednekar/litefunctions/operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewDeployment(function *apiv1.Function) *appsv1.Deployment {
	deploymentName := fmt.Sprintf("litefunctions-runtime-%s-%s-%s", function.Spec.Language, function.Spec.Project, function.Name)

	labels := map[string]string{
		"app":      "runtime",
		"lang":     function.Spec.Language,
		"project":  function.Spec.Project,
		"function": function.Spec.Name,
	}

	var image string
	switch function.Spec.Language {
	case "python":
		image = "ashupednekar535/litefunctions-runtime-py:latest"
	case "js":
		image = "ashupednekar535/litefunctions-runtime-js:latest"
	case "lua":
		image = "ashupednekar535/litefunctions-runtime-lua:latest"
	default:
		image = fmt.Sprintf("%s/%s/runtime-%s-%s-%s:latest", Cfg.Registry, Cfg.RegistryUser, function.Spec.Language, function.Spec.Project, function.Name)
	}

	return &appsv1.Deployment{
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
							Env: []corev1.EnvVar{
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
									Name:  "NATS_URL",
									Value: Cfg.NatsUrl,
								},
							},
						},
					},
				},
			},
		},
	}
}

func GetDeploymentName(function *apiv1.Function) string {
	return fmt.Sprintf("litefunctions-runtime-%s-%s-%s", function.Spec.Language, function.Spec.Project, function.Name)
}
