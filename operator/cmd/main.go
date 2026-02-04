package main

import (
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"

	appsv1 "github.com/ashupednekar/litefunctions/operator/api/v1"
	"github.com/spf13/cobra"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(appsv1.AddToScheme(scheme))
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "operator",
		Short: "LiteFunctions Operator",
	}

	rootCmd.AddCommand(managerCmd)
	rootCmd.AddCommand(cleanupCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
