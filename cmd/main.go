package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	smartschedulerv1 "github.com/kube-smartscheduler/smart-scheduler/api/v1"
	"github.com/kube-smartscheduler/smart-scheduler/controllers"
	"github.com/kube-smartscheduler/smart-scheduler/pkg/version"
	smartwebhook "github.com/kube-smartscheduler/smart-scheduler/webhook"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

// debugClient wraps a client.Client to log all API requests for debugging
type debugClient struct {
	client.Client
	debug bool
}

func (d *debugClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if d.debug {
		setupLog.Info("=== API REQUEST GET ===",
			"objectKind", obj.GetObjectKind(),
			"key", key,
			"namespace", key.Namespace,
			"name", key.Name)
	}
	err := d.Client.Get(ctx, key, obj, opts...)
	if d.debug {
		setupLog.Info("=== API RESPONSE GET ===",
			"objectKind", obj.GetObjectKind(),
			"key", key,
			"error", err,
			"resourceVersion", obj.GetResourceVersion())
	}
	return err
}

func (d *debugClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if d.debug {
		setupLog.Info("=== API REQUEST LIST ===",
			"objectKind", list.GetObjectKind())
	}
	err := d.Client.List(ctx, list, opts...)
	if d.debug {
		setupLog.Info("=== API RESPONSE LIST ===",
			"objectKind", list.GetObjectKind(),
			"error", err)
	}
	return err
}

func (d *debugClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if d.debug {
		setupLog.Info("=== API REQUEST CREATE ===",
			"objectKind", obj.GetObjectKind(),
			"namespace", obj.GetNamespace(),
			"name", obj.GetName())
	}
	err := d.Client.Create(ctx, obj, opts...)
	if d.debug {
		setupLog.Info("=== API RESPONSE CREATE ===",
			"objectKind", obj.GetObjectKind(),
			"namespace", obj.GetNamespace(),
			"name", obj.GetName(),
			"error", err)
	}
	return err
}

func (d *debugClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if d.debug {
		setupLog.Info("=== API REQUEST DELETE ===",
			"objectKind", obj.GetObjectKind(),
			"namespace", obj.GetNamespace(),
			"name", obj.GetName())
	}
	err := d.Client.Delete(ctx, obj, opts...)
	if d.debug {
		setupLog.Info("=== API RESPONSE DELETE ===",
			"objectKind", obj.GetObjectKind(),
			"namespace", obj.GetNamespace(),
			"name", obj.GetName(),
			"error", err)
	}
	return err
}

func (d *debugClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if d.debug {
		setupLog.Info("=== API REQUEST UPDATE ===",
			"objectKind", obj.GetObjectKind(),
			"namespace", obj.GetNamespace(),
			"name", obj.GetName(),
			"resourceVersion", obj.GetResourceVersion())
	}
	err := d.Client.Update(ctx, obj, opts...)
	if d.debug {
		setupLog.Info("=== API RESPONSE UPDATE ===",
			"objectKind", obj.GetObjectKind(),
			"namespace", obj.GetNamespace(),
			"name", obj.GetName(),
			"error", err)
	}
	return err
}

func (d *debugClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	if d.debug {
		setupLog.Info("=== API REQUEST PATCH ===",
			"objectKind", obj.GetObjectKind(),
			"namespace", obj.GetNamespace(),
			"name", obj.GetName(),
			"patchType", patch.Type())
	}
	err := d.Client.Patch(ctx, obj, patch, opts...)
	if d.debug {
		setupLog.Info("=== API RESPONSE PATCH ===",
			"objectKind", obj.GetObjectKind(),
			"namespace", obj.GetNamespace(),
			"name", obj.GetName(),
			"error", err)
	}
	return err
}

func (d *debugClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	if d.debug {
		setupLog.Info("=== API REQUEST DELETE_ALL_OF ===",
			"objectKind", obj.GetObjectKind())
	}
	err := d.Client.DeleteAllOf(ctx, obj, opts...)
	if d.debug {
		setupLog.Info("=== API RESPONSE DELETE_ALL_OF ===",
			"objectKind", obj.GetObjectKind(),
			"error", err)
	}
	return err
}

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(smartschedulerv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var webhookPort int
	var certDir string
	var enableDebugAPILogging bool
	var showVersion bool

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.IntVar(&webhookPort, "webhook-port", 9443, "The port the webhook server serves at.")
	flag.StringVar(&certDir, "cert-dir", "/tmp/k8s-webhook-server/serving-certs/", "The directory containing the webhook server certificates.")
	flag.BoolVar(&enableDebugAPILogging, "debug-api-requests", false, "Enable debug logging for all Kubernetes API requests.")
	flag.BoolVar(&showVersion, "version", false, "Show version information and exit.")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	// Handle version flag
	if showVersion {
		versionInfo := version.Get()
		fmt.Printf("Smart Scheduler Manager\n")
		fmt.Printf("%s\n", versionInfo.String())
		os.Exit(0)
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	versionInfo := version.Get()
	setupLog.Info("Starting Smart Scheduler Manager",
		"version", versionInfo.Version,
		"commit", versionInfo.CommitHash,
		"buildDate", versionInfo.BuildDate,
		"goVersion", versionInfo.GoVersion,
		"platform", versionInfo.Platform,
		"metricsAddr", metricsAddr,
		"probeAddr", probeAddr,
		"webhookPort", webhookPort,
		"enableLeaderElection", enableLeaderElection,
		"enableDebugAPILogging", enableDebugAPILogging)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		WebhookServer: webhook.NewServer(webhook.Options{
			Port:    webhookPort,
			CertDir: certDir,
		}),
		Metrics: server.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "smart-scheduler-leader",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Wrap client with debug logging if enabled
	var debugClientWrapper client.Client = mgr.GetClient()
	if enableDebugAPILogging {
		setupLog.Info("Debug API logging enabled - will log all Kubernetes API requests")
		debugClientWrapper = &debugClient{
			Client: mgr.GetClient(),
			debug:  true,
		}
	}

	// Setup controllers
	if err = (&controllers.SchedulerController{
		Client: debugClientWrapper,
		Scheme: mgr.GetScheme(),
		Log:    ctrl.Log.WithName("controllers").WithName("SchedulerController"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "SchedulerController")
		os.Exit(1)
	}

	// Setup webhook
	podMutator := &smartwebhook.PodMutator{
		Client: debugClientWrapper,
		Log:    ctrl.Log.WithName("webhook").WithName("PodMutator"),
	}

	if err = podMutator.SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to setup webhook", "webhook", "PodMutator")
		os.Exit(1)
	}

	// Setup RebalanceController
	if err = (&controllers.RebalanceController{
		Client: debugClientWrapper,
		Log:    ctrl.Log.WithName("controllers").WithName("RebalanceController"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "RebalanceController")
		os.Exit(1)
	}

	// Setup PodPlacementPolicyController
	if err = (&controllers.PodPlacementPolicyController{
		Client: debugClientWrapper,
		Log:    ctrl.Log.WithName("controllers").WithName("PodPlacementPolicyController"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PodPlacementPolicyController")
		os.Exit(1)
	}

	// Add health checks
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
