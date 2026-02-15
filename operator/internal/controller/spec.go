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
	deploymentName := GetDeploymentName(function)

	labels := map[string]string{
		"app":     "runtime",
		"lang":    function.Spec.Language,
		"project": function.Spec.Project,
	}
	if !isDynamicLanguage(function.Spec.Language) {
		labels["function"] = function.Spec.Name
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
		image = fmt.Sprintf("%s/%s/runtime-%s-%s-%s:latest", Cfg.Registry, Cfg.VcsUser, function.Spec.Language, function.Spec.Project, function.Name)
	}

	envVars := []corev1.EnvVar{
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
			Name:  "GIT_USER",
			Value: Cfg.VcsUser,
		},
		{
			Name:  "VCS_BASE_URL",
			Value: Cfg.VcsBaseUrl,
		},
		{
			Name:  "PROJECT",
			Value: function.Spec.Project,
		},
		{
			Name:  "REDIS_URL",
			Value: Cfg.RedisUrl,
		},
		{
			Name:  "REDIS_PASSWORD",
			Value: Cfg.RedisPassword,
		},
		{
			Name:  "NATS_URL",
			Value: Cfg.NatsUrl,
		},
		{
			Name:  "HTTP_PORT",
			Value: "8080",
		},
	}
	if !isDynamicLanguage(function.Spec.Language) {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "NAME",
			Value: function.Spec.Name,
		})
	}
	envVars = append(envVars, corev1.EnvVar{
		Name: "GIT_TOKEN",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: Cfg.GitTokenSecretName,
				},
				Key: Cfg.GitTokenSecretKey,
			},
		},
	})

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
							Ports: func() []corev1.ContainerPort {
								if supportsHTTP(function.Spec.Language) {
									return []corev1.ContainerPort{
										{
											Name:          "http",
											ContainerPort: 8080,
										},
									}
								}
								return nil
							}(),
							Env: envVars,
						},
					},
				},
			},
		},
	}
}

func GetDeploymentName(function *apiv1.Function) string {
	if isDynamicLanguage(function.Spec.Language) {
		return fmt.Sprintf("litefunctions-runtime-%s-%s", function.Spec.Language, function.Spec.Project)
	}
	return fmt.Sprintf("litefunctions-runtime-%s-%s-%s", function.Spec.Language, function.Spec.Project, function.Name)
}

func GetServiceName(function *apiv1.Function) string {
	if isDynamicLanguage(function.Spec.Language) {
		return fmt.Sprintf("litefunctions-runtime-svc-%s-%s", function.Spec.Language, function.Spec.Project)
	}
	return fmt.Sprintf("litefunctions-runtime-svc-%s-%s-%s", function.Spec.Language, function.Spec.Project, function.Name)
}

func NewService(function *apiv1.Function) *corev1.Service {
	labels := map[string]string{
		"app":     "runtime",
		"lang":    function.Spec.Language,
		"project": function.Spec.Project,
	}
	if !isDynamicLanguage(function.Spec.Language) {
		labels["function"] = function.Spec.Name
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetServiceName(function),
			Namespace: function.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Name: "http",
					Port: 8080,
				},
			},
		},
	}
}

func supportsHTTP(lang string) bool {
	switch lang {
	case "go", "rust", "rs", "python":
		return true
	default:
		return false
	}
}

func isDynamicLanguage(lang string) bool {
	switch lang {
	case "python", "js", "lua":
		return true
	default:
		return false
	}
}
