package trustmanager

import (
	"context"
	"fmt"
	"reflect"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"

	v1alpha1 "github.com/openshift/cert-manager-operator/api/operator/v1alpha1"
)

var (
	requestEnqueueLabelKey   = "app"
	requestEnqueueLabelValue = trustManagerCommonName
)

// +kubebuilder:rbac:groups=operator.openshift.io,resources=trustmanagers,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=operator.openshift.io,resources=trustmanagers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.openshift.io,resources=trustmanagers/finalizers,verbs=update
// +kubebuilder:rbac:groups=cert-manager.io,resources=certificates;issuers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services;serviceaccounts;namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;clusterrolebindings;roles;rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingwebhookconfigurations,verbs=get;list;watch;create;update;patch;delete

// Reconciler reconciles a TrustManager object.
type Reconciler struct {
	ctrlClient

	ctx           context.Context
	eventRecorder record.EventRecorder
	log           logr.Logger
	scheme        *runtime.Scheme
}

// NewCacheBuilder returns a cache builder configured for trust-manager managed resources.
func NewCacheBuilder(config *rest.Config, opts cache.Options) (cache.Cache, error) {
	labelReq, err := labels.NewRequirement(requestEnqueueLabelKey, selection.Equals, []string{requestEnqueueLabelValue})
	if err != nil {
		return nil, fmt.Errorf("invalid cache label requirement: %w", err)
	}
	selector := labels.NewSelector().Add(*labelReq)
	opts.ByObject = map[client.Object]cache.ByObject{
		&v1alpha1.TrustManager{}:     {},
		&certmanagerv1.Certificate{}: {Label: selector},
		&certmanagerv1.Issuer{}:      {Label: selector},
		&appsv1.Deployment{}:         {Label: selector},
		&rbacv1.ClusterRole{}:        {Label: selector},
		&rbacv1.ClusterRoleBinding{}: {Label: selector},
		&rbacv1.Role{}:               {Label: selector},
		&rbacv1.RoleBinding{}:        {Label: selector},
		&corev1.Service{}:            {Label: selector},
		&corev1.ServiceAccount{}:     {Label: selector},
		&admissionregistrationv1.ValidatingWebhookConfiguration{}: {Label: selector},
	}
	return cache.New(config, opts)
}

// New returns a new Reconciler instance.
func New(mgr ctrl.Manager) (*Reconciler, error) {
	c, err := NewClient(mgr)
	if err != nil {
		return nil, err
	}
	return &Reconciler{
		ctrlClient:    c,
		ctx:           context.Background(),
		eventRecorder: mgr.GetEventRecorderFor(ControllerName),
		log:           ctrl.Log.WithName(ControllerName),
		scheme:        mgr.GetScheme(),
	}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	mapFunc := func(ctx context.Context, obj client.Object) []reconcile.Request {
		if obj.GetLabels() == nil || obj.GetLabels()[requestEnqueueLabelKey] != requestEnqueueLabelValue {
			return nil
		}
		return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: trustManagerObjectName}}}
	}
	managed := predicate.NewPredicateFuncs(func(object client.Object) bool {
		return object.GetLabels() != nil && object.GetLabels()[requestEnqueueLabelKey] == requestEnqueueLabelValue
	})
	withIgnoreStatus := builder.WithPredicates(predicate.GenerationChangedPredicate{}, managed)
	managedOnly := builder.WithPredicates(managed)

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.TrustManager{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Named(ControllerName).
		Watches(&certmanagerv1.Certificate{}, handler.EnqueueRequestsFromMapFunc(mapFunc), withIgnoreStatus).
		Watches(&certmanagerv1.Issuer{}, handler.EnqueueRequestsFromMapFunc(mapFunc), withIgnoreStatus).
		Watches(&appsv1.Deployment{}, handler.EnqueueRequestsFromMapFunc(mapFunc), withIgnoreStatus).
		Watches(&rbacv1.ClusterRole{}, handler.EnqueueRequestsFromMapFunc(mapFunc), managedOnly).
		Watches(&rbacv1.ClusterRoleBinding{}, handler.EnqueueRequestsFromMapFunc(mapFunc), managedOnly).
		Watches(&rbacv1.Role{}, handler.EnqueueRequestsFromMapFunc(mapFunc), managedOnly).
		Watches(&rbacv1.RoleBinding{}, handler.EnqueueRequestsFromMapFunc(mapFunc), managedOnly).
		Watches(&corev1.Service{}, handler.EnqueueRequestsFromMapFunc(mapFunc), managedOnly).
		Watches(&corev1.ServiceAccount{}, handler.EnqueueRequestsFromMapFunc(mapFunc), managedOnly).
		Watches(&admissionregistrationv1.ValidatingWebhookConfiguration{}, handler.EnqueueRequestsFromMapFunc(mapFunc), managedOnly).
		Complete(r)
}

// Reconcile compares desired TrustManager state with the cluster and reconciles operand resources.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log.V(1).Info("reconciling", "request", req)

	tm := &v1alpha1.TrustManager{}
	if err := r.Get(ctx, req.NamespacedName, tm); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to fetch trustmanager %q: %w", req.NamespacedName, err)
	}

	if !tm.DeletionTimestamp.IsZero() {
		if err := r.removeFinalizer(ctx, tm); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if err := r.addFinalizer(ctx, tm); err != nil {
		return ctrl.Result{}, err
	}

	if !isManagementEnabled(tm) {
		setReadyConditions(tm, metav1.ConditionFalse, v1alpha1.ReasonReady, "trust-manager managementState is not Enabled")
		if err := r.updateStatus(ctx, tm); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	createRecon := !containsProcessedAnnotation(tm) && reflect.DeepEqual(tm.Status, v1alpha1.TrustManagerStatus{})
	if err := r.reconcileTrustManagerOperand(tm, createRecon); err != nil {
		r.log.Error(err, "failed to reconcile trust-manager operand", "name", tm.GetName())
		if IsIrrecoverableError(err) {
			tm.Status.SetCondition(v1alpha1.Degraded, metav1.ConditionTrue, v1alpha1.ReasonFailed, err.Error())
			tm.Status.SetCondition(v1alpha1.Ready, metav1.ConditionFalse, v1alpha1.ReasonReady, "")
		} else {
			tm.Status.SetCondition(v1alpha1.Degraded, metav1.ConditionFalse, v1alpha1.ReasonInProgress, err.Error())
			tm.Status.SetCondition(v1alpha1.Ready, metav1.ConditionFalse, v1alpha1.ReasonInProgress, err.Error())
		}
		_ = r.updateStatus(ctx, tm)
		if IsRetryRequiredError(err) {
			return ctrl.Result{RequeueAfter: defaultRequeueTime}, nil
		}
		return ctrl.Result{}, err
	}

	setReadyConditions(tm, metav1.ConditionTrue, v1alpha1.ReasonReady, "trust-manager operand reconciled successfully")
	if err := r.updateStatus(ctx, tm); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}
