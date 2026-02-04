package client

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiv1 "github.com/ashupednekar/litefunctions/operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

type Client struct {
	Client client.Client
	Log    logr.Logger
	Cfg    *Config
}

type Config struct {
	Registry     string
	RegistryUser string
	PullSecret   string
	DbSecretName string
	DbSecretKey  string
	RedisUrl     string
	NatsUrl      string
}

func NewClient(c client.Client, log logr.Logger, cfg *Config) *Client {
	return &Client{
		Client: c,
		Log:    log,
		Cfg:    cfg,
	}
}

func (c *Client) GetFunction(ctx context.Context, namespace, name string) (*apiv1.Function, error) {
	var function apiv1.Function
	if err := c.Client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &function); err != nil {
		return nil, fmt.Errorf("failed to get function: %w", err)
	}
	return &function, nil
}

func (c *Client) IsFunctionActive(ctx context.Context, namespace, name string) (bool, error) {
	function, err := c.GetFunction(ctx, namespace, name)
	if err != nil {
		return false, err
	}
	c.Log.V(1).Info("Checked function active status", "namespace", namespace, "name", name, "active", function.Spec.IsActive)
	return function.Spec.IsActive, nil
}

func (c *Client) MarkFunctionActive(ctx context.Context, namespace, name string, keepWarmDuration time.Duration) (string, error) {
	function, err := c.GetFunction(ctx, namespace, name)
	if err != nil {
		return "", err
	}

	function.Spec.IsActive = true
	now := time.Now()
	deprovisionTime := now.Add(keepWarmDuration)
	function.Spec.DeProvisionTime = deprovisionTime.Format(time.RFC3339)

	if err := c.Client.Update(ctx, function); err != nil {
		return "", fmt.Errorf("failed to update function: %w", err)
	}

	c.Log.Info("Marked function as active", "namespace", namespace, "name", name, "deprovisionTime", deprovisionTime)
	return function.Spec.Language, nil
}

func (c *Client) ExtendFunctionLease(ctx context.Context, namespace, name string, keepWarmDuration time.Duration) (bool, error) {
	function, err := c.GetFunction(ctx, namespace, name)
	if err != nil {
		return false, err
	}

	if !function.Spec.IsActive {
		c.Log.V(1).Info("Function inactive, lease not extended", "namespace", namespace, "name", name)
		return false, nil
	}

	deprovisionTime := time.Now().Add(keepWarmDuration)
	function.Spec.DeProvisionTime = deprovisionTime.Format(time.RFC3339)

	if err := c.Client.Update(ctx, function); err != nil {
		return false, fmt.Errorf("failed to extend function lease: %w", err)
	}

	c.Log.V(1).Info("Extended function lease", "namespace", namespace, "name", name, "deprovisionTime", deprovisionTime)
	return true, nil
}

func (c *Client) MarkFunctionInactive(ctx context.Context, namespace, name string) error {
	function, err := c.GetFunction(ctx, namespace, name)
	if err != nil {
		return err
	}

	if !function.Spec.IsActive {
		c.Log.V(1).Info("Function already inactive", "namespace", namespace, "name", name)
		return nil
	}

	function.Spec.IsActive = false
	if err := c.Client.Update(ctx, function); err != nil {
		return fmt.Errorf("failed to update function: %w", err)
	}

	c.Log.Info("Marked function as inactive", "namespace", namespace, "name", name)
	return nil
}

func (c *Client) CreateFunctionIfNotExists(ctx context.Context, namespace, name, project, language, gitCreds string) (bool, error) {
	var existing apiv1.Function
	err := c.Client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &existing)
	if err == nil {
		c.Log.V(1).Info("Function already exists", "namespace", namespace, "name", name)
		return false, nil
	}
	if !apierrors.IsNotFound(err) {
		return false, fmt.Errorf("failed to check function: %w", err)
	}

	function := &apiv1.Function{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: apiv1.FunctionSpec{
			IsActive:        false,
			DeProvisionTime: "",
			Language:        language,
			Name:            name,
			Project:         project,
			GitCreds:        gitCreds,
		},
	}

	if err := c.Client.Create(ctx, function); err != nil {
		return false, fmt.Errorf("failed to create function: %w", err)
	}

	c.Log.Info("Created function", "namespace", namespace, "name", name, "project", project)
	return true, nil
}

func (c *Client) CreateOrUpdateDeployment(ctx context.Context, function *apiv1.Function) error {
	deployment := c.NewDeployment(function)
	deploymentName := GetDeploymentName(function)

	var existing appsv1.Deployment
	err := c.Client.Get(ctx, client.ObjectKey{Name: deploymentName, Namespace: function.Namespace}, &existing)

	if err != nil {
		if err := c.Client.Create(ctx, deployment); err != nil {
			return fmt.Errorf("failed to create deployment: %w", err)
		}
		c.Log.Info("Created deployment for function", "deployment", deploymentName)
		return nil
	}

	deployment.ResourceVersion = existing.ResourceVersion
	if err := c.Client.Update(ctx, deployment); err != nil {
		return fmt.Errorf("failed to update deployment: %w", err)
	}
	c.Log.Info("Updated deployment for function", "deployment", deploymentName)
	return nil
}

func (c *Client) DeleteDeployment(ctx context.Context, namespace, name string) error {
	var deployment appsv1.Deployment
	if err := c.Client.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, &deployment); err != nil {
		return client.IgnoreNotFound(err)
	}

	if err := c.Client.Delete(ctx, &deployment); err != nil {
		return fmt.Errorf("failed to delete deployment: %w", err)
	}
	c.Log.Info("Deleted deployment", "deployment", name)
	return nil
}

func (c *Client) NewDeployment(function *apiv1.Function) *appsv1.Deployment {
	deploymentName := GetDeploymentName(function)

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
		image = fmt.Sprintf("%s/%s/runtime-%s-%s-%s:latest", c.Cfg.Registry, c.Cfg.RegistryUser, function.Spec.Language, function.Spec.Project, function.Name)
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: function.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To[int32](1),
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					ImagePullSecrets: []corev1.LocalObjectReference{
						{Name: c.Cfg.PullSecret},
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
												Name: c.Cfg.DbSecretName,
											},
											Key: c.Cfg.DbSecretKey,
										},
									},
								},
								{
									Name:  "REDIS_URL",
									Value: c.Cfg.RedisUrl,
								},
								{
									Name:  "NATS_URL",
									Value: c.Cfg.NatsUrl,
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
