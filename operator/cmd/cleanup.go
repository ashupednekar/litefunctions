package main

import (
	"context"
	"os"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	appsv1 "github.com/ashupednekar/litefunctions/operator/api/v1"
	"github.com/ashupednekar/litefunctions/operator/internal/controller"
	"github.com/spf13/cobra"
	k8sappsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Cleanup inactive functions",
	Run: func(cmd *cobra.Command, args []string) {
		runCleanup(cmd)
	},
}

func init() {
	cleanupCmd.Flags().String("namespace", "litefunctions", "Namespace to cleanup functions in.")
}

func runCleanup(cmd *cobra.Command) {
	namespace, _ := cmd.Flags().GetString("namespace")

	opts := zap.Options{
		Development: true,
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	log := ctrl.Log.WithName("cleanup")

	controller.LoadCfg(log)

	config := ctrl.GetConfigOrDie()
	k8sClient, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		log.Error(err, "Failed to create k8s client")
		os.Exit(1)
	}

	ctx := context.Background()

	var functionList appsv1.FunctionList
	if err := k8sClient.List(ctx, &functionList, client.InNamespace(namespace)); err != nil {
		log.Error(err, "Failed to list functions")
		os.Exit(1)
	}

	now := time.Now().UTC()

	for _, function := range functionList.Items { //TODO: consider concurrent later
		if function.Spec.IsActive && function.Spec.DeProvisionTime != "" {
			deprovisionTime, err := time.Parse(time.RFC3339, function.Spec.DeProvisionTime)
			if err != nil {
				log.Error(err, "Failed to parse deprovision time", "function", function.Name)
				continue
			}

			if now.After(deprovisionTime) {
				log.Info("Deactivating function", "function", function.Name, "deprovisionTime", deprovisionTime)

				patch := client.MergeFrom(function.DeepCopy())
				function.Spec.IsActive = false

				if err := k8sClient.Patch(ctx, &function, patch); err != nil {
					log.Error(err, "Failed to patch function", "function", function.Name)
				} else {
					log.Info("Successfully deactivated function", "function", function.Name)
				}

				sharedInUse, err := hasOtherActiveFunctionUsingSharedRuntime(ctx, k8sClient, &function)
				if err != nil {
					log.Error(err, "Failed to check shared runtime usage", "function", function.Name)
					continue
				}
				if sharedInUse {
					log.Info(
						"Skipping shared runtime cleanup; another active function still uses it",
						"project", function.Spec.Project,
						"language", function.Spec.Language,
					)
					continue
				}

				deploymentName := controller.GetDeploymentName(&function)
				var existingDeploy k8sappsv1.Deployment
				deployErr := k8sClient.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: function.Namespace}, &existingDeploy)
				if deployErr == nil {
					if err := k8sClient.Delete(ctx, &existingDeploy); err != nil {
						log.Error(err, "Failed to delete deployment", "deployment", deploymentName)
					} else {
						log.Info("Deleted deployment for inactive function", "deployment", deploymentName)
					}
				} else if !apierrs.IsNotFound(deployErr) {
					log.Error(deployErr, "Failed to get deployment", "deployment", deploymentName)
				}

				serviceName := controller.GetServiceName(&function)
				var existingSvc corev1.Service
				svcErr := k8sClient.Get(ctx, types.NamespacedName{Name: serviceName, Namespace: function.Namespace}, &existingSvc)
				if svcErr == nil {
					if err := k8sClient.Delete(ctx, &existingSvc); err != nil {
						log.Error(err, "Failed to delete service", "service", serviceName)
					} else {
						log.Info("Deleted service for inactive function", "service", serviceName)
					}
				} else if !apierrs.IsNotFound(svcErr) {
					log.Error(svcErr, "Failed to get service", "service", serviceName)
				}
			}
		}
	}

	log.Info("Cleanup completed")
}

func hasOtherActiveFunctionUsingSharedRuntime(ctx context.Context, k8sClient client.Client, function *appsv1.Function) (bool, error) {
	if !isDynamicLanguage(function.Spec.Language) {
		return false, nil
	}

	var functionList appsv1.FunctionList
	if err := k8sClient.List(ctx, &functionList, client.InNamespace(function.Namespace)); err != nil {
		return false, err
	}

	for i := range functionList.Items {
		fn := &functionList.Items[i]
		if fn.Name == function.Name {
			continue
		}
		if fn.Spec.Project != function.Spec.Project || fn.Spec.Language != function.Spec.Language {
			continue
		}
		if fn.Spec.IsActive {
			return true, nil
		}
	}

	return false, nil
}

func isDynamicLanguage(lang string) bool {
	switch lang {
	case "python", "ts", "lua":
		return true
	default:
		return false
	}
}
