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
			}
		}
	}

	log.Info("Cleanup completed")
}
