package main

import (
	"flag"
	"net/http"
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/darkowlzz/operator-toolkit/webhook/cert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "e990bcb8.my.domain",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Create an uncached client to be used to be used by the certificate
	// manager. Cached client isn't ready for use at this stage.
	cli, err := client.New(mgr.GetConfig(), client.Options{Scheme: mgr.GetScheme()})
	if err != nil {
		setupLog.Error(err, "failed to create raw client")
		os.Exit(1)
	}
	// Configure the certificate manager.
	certOpts := cert.Options{
		Service: &admissionregistrationv1.ServiceReference{
			Name:      "webhook-service",
			Namespace: "default",
		},
		Client:    cli,
		SecretRef: &types.NamespacedName{Name: "webhook-secret", Namespace: "default"},
	}
	if err := cert.NewManager(nil, certOpts); err != nil {
		setupLog.Error(err, "unable to provision certificate")
		os.Exit(1)
	}

	newFH := FooHandler{Name: "kubebuilder"}

	// NOTE: Sometimes web browsers automatically add a trailing slash to the
	// address and that results in a 404 when the registered path has no
	// trailing slash. Registering with trailing slash is safer.

	// Custom handler.
	mgr.GetWebhookServer().Register("/foo/", newFH)

	// File server handler. This helps serve all the static files - html, css,
	// js, etc.
	staticfs := http.StripPrefix("/baz/", http.FileServer(http.Dir("./static")))
	mgr.GetWebhookServer().Register("/baz/", staticfs)

	//+kubebuilder:scaffold:builder

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

type FooHandler struct {
	Name string
}

func (fh FooHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Write to the response write directly.
	// fmt.Fprintf(w, "Hello %s", fh.Name)

	// Serve webpage.
	// http.ServeFile(w, r, "static/index.html")
	http.ServeFile(w, r, "static/")
}
