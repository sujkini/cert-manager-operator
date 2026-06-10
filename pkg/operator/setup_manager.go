package operator

import (
	"context"
	"fmt"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"

	v1alpha1 "github.com/openshift/cert-manager-operator/api/operator/v1alpha1"
	"github.com/openshift/cert-manager-operator/pkg/controller/istiocsr"
	"github.com/openshift/cert-manager-operator/pkg/controller/trustmanager"
	"github.com/openshift/cert-manager-operator/pkg/version"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup-manager")
)

func init() {
	ctrllog.SetLogger(klog.NewKlogr())

	utilruntime.Must(clientscheme.AddToScheme(scheme))
	utilruntime.Must(appsv1.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(networkingv1.AddToScheme(scheme))
	utilruntime.Must(rbacv1.AddToScheme(scheme))
	utilruntime.Must(admissionregistrationv1.AddToScheme(scheme))
	utilruntime.Must(certmanagerv1.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
}

// ControllerManagerConfig selects which addon controllers to run in the shared manager.
type ControllerManagerConfig struct {
	EnableIstioCSR     bool
	EnableTrustManager bool
}

// Manager holds the shared controller-runtime manager for addon controllers.
type Manager struct {
	manager manager.Manager
}

func newCombinedCacheBuilder(cfg ControllerManagerConfig) func(*rest.Config, cache.Options) (cache.Cache, error) {
	labelValues := []string{}
	if cfg.EnableIstioCSR {
		labelValues = append(labelValues, "cert-manager-istio-csr")
	}
	if cfg.EnableTrustManager {
		labelValues = append(labelValues, "cert-manager-trust-manager")
	}
	return func(config *rest.Config, opts cache.Options) (cache.Cache, error) {
		if len(labelValues) == 1 && cfg.EnableIstioCSR && !cfg.EnableTrustManager {
			return istiocsr.NewCacheBuilder(config, opts)
		}
		if len(labelValues) == 1 && cfg.EnableTrustManager && !cfg.EnableIstioCSR {
			return trustmanager.NewCacheBuilder(config, opts)
		}

		op := selection.In
		if len(labelValues) == 1 {
			op = selection.Equals
		}
		labelReq, err := labels.NewRequirement("app", op, labelValues)
		if err != nil {
			return nil, fmt.Errorf("invalid cache label requirement: %w", err)
		}
		selector := labels.NewSelector().Add(*labelReq)
		byObject := map[client.Object]cache.ByObject{
			&certmanagerv1.Certificate{}:  {Label: selector},
			&certmanagerv1.Issuer{}:       {Label: selector},
			&appsv1.Deployment{}:          {Label: selector},
			&rbacv1.ClusterRole{}:         {Label: selector},
			&rbacv1.ClusterRoleBinding{}:  {Label: selector},
			&rbacv1.Role{}:                {Label: selector},
			&rbacv1.RoleBinding{}:         {Label: selector},
			&corev1.Service{}:             {Label: selector},
			&corev1.ServiceAccount{}:      {Label: selector},
			&networkingv1.NetworkPolicy{}: {Label: selector},
			&admissionregistrationv1.ValidatingWebhookConfiguration{}: {Label: selector},
		}
		if cfg.EnableIstioCSR {
			byObject[&v1alpha1.IstioCSR{}] = cache.ByObject{}
		}
		if cfg.EnableTrustManager {
			byObject[&v1alpha1.TrustManager{}] = cache.ByObject{}
		}
		opts.ByObject = byObject
		return cache.New(config, opts)
	}
}

// NewControllerManager creates a shared manager for enabled addon controllers.
func NewControllerManager(cfg ControllerManagerConfig) (*Manager, error) {
	setupLog.Info("setting up operator manager", "istioCSR", cfg.EnableIstioCSR, "trustManager", cfg.EnableTrustManager)
	setupLog.Info("controller", "version", version.Get())

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:   scheme,
		NewCache: newCombinedCacheBuilder(cfg),
		Logger:   ctrl.Log.WithName("operator-manager"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create manager: %w", err)
	}

	if cfg.EnableIstioCSR {
		r, err := istiocsr.New(mgr)
		if err != nil {
			return nil, fmt.Errorf("failed to create %s reconciler: %w", istiocsr.ControllerName, err)
		}
		if err := r.SetupWithManager(mgr); err != nil {
			return nil, fmt.Errorf("failed to setup %s controller: %w", istiocsr.ControllerName, err)
		}
	}
	if cfg.EnableTrustManager {
		r, err := trustmanager.New(mgr)
		if err != nil {
			return nil, fmt.Errorf("failed to create %s reconciler: %w", trustmanager.ControllerName, err)
		}
		if err := r.SetupWithManager(mgr); err != nil {
			return nil, fmt.Errorf("failed to setup %s controller: %w", trustmanager.ControllerName, err)
		}
	}

	return &Manager{manager: mgr}, nil
}

// Start starts the operator synchronously until a message is received from ctx.
func (mgr *Manager) Start(ctx context.Context) error {
	return mgr.manager.Start(ctx)
}
