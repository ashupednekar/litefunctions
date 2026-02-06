package main

import (
	"crypto/tls"
	"net"
	"os"
	"path/filepath"
	"strings"

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/certwatcher"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	appsv1 "github.com/ashupednekar/litefunctions/operator/api/v1"
	"github.com/ashupednekar/litefunctions/operator/internal/client"
	"github.com/ashupednekar/litefunctions/operator/internal/controller"
	functionserver "github.com/ashupednekar/litefunctions/operator/internal/server"
	functionproto "github.com/ashupednekar/litefunctions/common/proto"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
}

var managerCmd = &cobra.Command{
	Use:   "manager",
	Short: "Run as operator manager",
	Run: func(cmd *cobra.Command, args []string) {
		if err := runManager(cmd); err != nil {
			os.Exit(1)
		}
	},
}

func init() {
	managerCmd.Flags().String("metrics-bind-address", "0", "The address the metrics endpoint binds to.")
	managerCmd.Flags().Bool("leader-elect", false, "Enable leader election for controller manager.")
	managerCmd.Flags().Bool("metrics-secure", true, "Enable metrics secure serving.")
	managerCmd.Flags().String("webhook-cert-path", "", "Directory that contains webhook certificate.")
	managerCmd.Flags().String("webhook-cert-name", "tls.crt", "Name of webhook certificate file.")
	managerCmd.Flags().String("webhook-cert-key", "tls.key", "Name of webhook key file.")
	managerCmd.Flags().String("metrics-cert-path", "", "Directory that contains metrics server certificate.")
	managerCmd.Flags().String("metrics-cert-name", "tls.crt", "Name of metrics server certificate file.")
	managerCmd.Flags().String("metrics-cert-key", "tls.key", "Name of metrics server key file.")
	managerCmd.Flags().Bool("enable-http2", false, "Enable HTTP/2 for metrics and webhook servers.")
}

func runManager(cmd *cobra.Command) error {
	metricsAddr, _ := cmd.Flags().GetString("metrics-bind-address")
	enableLeaderElection, _ := cmd.Flags().GetBool("leader-elect")
	secureMetrics, _ := cmd.Flags().GetBool("metrics-secure")
	webhookCertPath, _ := cmd.Flags().GetString("webhook-cert-path")
	webhookCertName, _ := cmd.Flags().GetString("webhook-cert-name")
	webhookCertKey, _ := cmd.Flags().GetString("webhook-cert-key")
	metricsCertPath, _ := cmd.Flags().GetString("metrics-cert-path")
	metricsCertName, _ := cmd.Flags().GetString("metrics-cert-name")
	metricsCertKey, _ := cmd.Flags().GetString("metrics-cert-key")
	enableHTTP2, _ := cmd.Flags().GetBool("enable-http2")

	var tlsOpts []func(*tls.Config)
	opts := zap.Options{
		Development: true,
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	var metricsCertWatcher, webhookCertWatcher *certwatcher.CertWatcher

	webhookTLSOpts := tlsOpts

	if len(webhookCertPath) > 0 {
		setupLog.Info("Initializing webhook certificate watcher", "webhook-cert-path", webhookCertPath, "webhook-cert-name", webhookCertName, "webhook-cert-key", webhookCertKey)

		var err error
		webhookCertWatcher, err = certwatcher.New(
			filepath.Join(webhookCertPath, webhookCertName),
			filepath.Join(webhookCertPath, webhookCertKey),
		)
		if err != nil {
			setupLog.Error(err, "Failed to initialize webhook certificate watcher")
			return err
		}

		webhookTLSOpts = append(webhookTLSOpts, func(config *tls.Config) {
			config.GetCertificate = webhookCertWatcher.GetCertificate
		})
	}

	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: webhookTLSOpts,
	})

	metricsServerOptions := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
		TLSOpts:       tlsOpts,
	}

	if secureMetrics {
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	if len(metricsCertPath) > 0 {
		setupLog.Info("Initializing metrics certificate watcher", "metrics-cert-path", metricsCertPath, "metrics-cert-name", metricsCertName, "metrics-cert-key", metricsCertKey)

		var err error
		metricsCertWatcher, err = certwatcher.New(
			filepath.Join(metricsCertPath, metricsCertName),
			filepath.Join(metricsCertPath, metricsCertKey),
		)
		if err != nil {
			setupLog.Error(err, "Failed to initialize metrics certificate")
			return err
		}

		metricsServerOptions.TLSOpts = append(metricsServerOptions.TLSOpts, func(config *tls.Config) {
			config.GetCertificate = metricsCertWatcher.GetCertificate
		})
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsServerOptions,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: "0",
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "85c94c56.ashupednekar.github.io",
	})
	if err != nil {
		setupLog.Error(err, "Unable to start manager")
		return err
	}

	if err := (&controller.FunctionReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Unable to create controller", "controller", "Function")
		return err
	}

	controller.LoadCfg(setupLog)

	cfg := &client.Config{
		Registry:      controller.Cfg.Registry,
		RegistryUser:  controller.Cfg.RegistryUser,
		PullSecret:    controller.Cfg.PullSecret,
		DbSecretName:  controller.Cfg.DbSecretName,
		DbSecretKey:   controller.Cfg.DbSecretKey,
		RedisUrl:      controller.Cfg.RedisUrl,
		RedisPassword: controller.Cfg.RedisPassword,
		NatsUrl:       controller.Cfg.NatsUrl,
	}

	k8sClient := client.NewClient(mgr.GetClient(), setupLog, cfg)

	grpcPort := strings.TrimSpace(os.Getenv("GRPC_PORT"))
	if grpcPort == "" {
		grpcPort = "50051"
	}
	grpcAddr := ":" + grpcPort
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		setupLog.Error(err, "Failed to listen for gRPC", "address", grpcAddr)
		return err
	}

	grpcServer := grpc.NewServer()
	functionproto.RegisterFunctionServiceServer(grpcServer, functionserver.NewFunctionServer(k8sClient, setupLog, controller.Cfg.KeepWarmDuration))

	go func() {
		setupLog.Info("Starting gRPC server", "address", grpcAddr)
		if err := grpcServer.Serve(lis); err != nil {
			setupLog.Error(err, "gRPC server failed")
		}
	}()

	setupLog.Info("Starting manager with gRPC server")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "Problem running manager")
		return err
	}
	return nil
}
