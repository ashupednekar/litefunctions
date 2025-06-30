package startup

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)


func SetupIngestor(c client.Client, ns string) error {
	ctx := context.Background()
	log := logf.FromContext(ctx)
	log.Info("commencing ingestor setup")
  labels := map[string]string{
		"operator": "litefunctions",
		"component": "ingestor",
	}
	name := "litefunctions-ingestor"
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Namespace: ns,
			Labels: labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32(1),
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels, Namespace: ns},
				Spec: corev1.PodSpec{
					ImagePullSecrets: []corev1.LocalObjectReference{
						{Name: "ghcr-secret"},
					},
					Containers: []corev1.Container{
						{
							Name: "ingestor",
							Image: "ashupednekar535/litefunctions-ingestor:latest",
							ImagePullPolicy: corev1.PullAlways,
							Env: []corev1.EnvVar{
							},
						},
					},
				},
			},
		},
	}

	var existing appsv1.Deployment
	err := c.Get(ctx, types.NamespacedName{Name: ""}, &existing)
	if err != nil && apierrs.IsNotFound(err) {
		if err := c.Create(ctx, deploy); err != nil {
			log.Error(err, "Failed to create deployment", "deployment", name)
			return err
		}
		log.Info("Created new deployment for function", "deployment", name)
	} else if err == nil {
		deploy.ResourceVersion = existing.ResourceVersion
		if err := c.Update(ctx, deploy); err != nil {
			log.Error(err, "Failed to update deployment", "deployment", name)
			return err
		}
		log.Info("Updated existing deployment for function", "deployment", name)
	} else {
		log.Error(err, "Failed to get deployment", "deployment", name)
		return err
	}
	

  service := &corev1.Service{
  	ObjectMeta: metav1.ObjectMeta{
  		Name:   "litefunctions-ingestor",
			Namespace: ns,
  		Labels: labels,
  	},
  	Spec: corev1.ServiceSpec{
  		Selector: labels,
  		Ports: []corev1.ServicePort{
  			{
  				Name:       "http",
  				Port:       3000,
  				TargetPort: intstr.FromInt(3000),
  			},
  		},
  		Type: corev1.ServiceTypeNodePort,
  	},
  }
	

  var existingSvc corev1.Service
  svcKey := types.NamespacedName{Name: name}
  err = c.Get(ctx, svcKey, &existingSvc)
  if err != nil && apierrs.IsNotFound(err) {
  	if err := c.Create(ctx, service); err != nil {
  		log.Error(err, "Failed to create service", "service", service.Name)
  		return err
  	}
  	log.Info("Created new service for function", "service", service.Name)
  } else if err == nil {
  	service.ResourceVersion = existingSvc.ResourceVersion
  	if err := c.Update(ctx, service); err != nil {
  		log.Error(err, "Failed to update service", "service", service.Name)
  		return err
  	}
  	log.Info("Updated existing service for function", "service", service.Name)
  } else {
  	log.Error(err, "Failed to get service", "service", service.Name)
  	return err
  }
	
	return nil
}
