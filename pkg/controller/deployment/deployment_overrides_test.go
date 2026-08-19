package deployment

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"

	"github.com/openshift/cert-manager-operator/api/operator/v1alpha1"
	"github.com/openshift/cert-manager-operator/pkg/operator/assets"
	"github.com/openshift/cert-manager-operator/pkg/operator/clientset/versioned/fake"
	certmanoperatorinformer "github.com/openshift/cert-manager-operator/pkg/operator/informers/externalversions"
	certmanagerinformer "github.com/openshift/cert-manager-operator/pkg/operator/informers/externalversions/operator/v1alpha1"
)

func TestUnsupportedConfigOverrides(t *testing.T) {
	deploymentAssetPaths := map[string]string{
		"cert-manager":            "cert-manager-deployment/controller/cert-manager-deployment.yaml",
		"cert-manager-cainjector": "cert-manager-deployment/cainjector/cert-manager-cainjector-deployment.yaml",
		"cert-manager-webhook":    "cert-manager-deployment/webhook/cert-manager-webhook-deployment.yaml",
	}
	deployments := make(map[string]*appsv1.Deployment)
	for deploymentName, assetPath := range deploymentAssetPaths {
		manifestFile, err := assets.Asset(assetPath)
		require.NoError(t, err)
		deployments[deploymentName] = resourceread.ReadDeploymentV1OrDie(manifestFile)
	}

	defaultDeploymentArgs := map[string][]string{
		"cert-manager": {
			"--v=2",
			"--cluster-resource-namespace=$(POD_NAMESPACE)",
			"--leader-election-namespace=kube-system",
			"--acme-http01-solver-image=quay.io/jetstack/cert-manager-acmesolver:v1.18.3",
			"--max-concurrent-challenges=60",
			"--feature-gates=ACMEHTTP01IngressPathTypeExact=false",
		},
		"cert-manager-cainjector": {
			"--v=2",
			"--leader-election-namespace=kube-system",
		},
		"cert-manager-webhook": {
			"--v=2",
			"--secure-port=10250",
			"--dynamic-serving-ca-secret-namespace=$(POD_NAMESPACE)",
			"--dynamic-serving-ca-secret-name=cert-manager-webhook-ca",
			"--dynamic-serving-dns-names=cert-manager-webhook,cert-manager-webhook.$(POD_NAMESPACE),cert-manager-webhook.$(POD_NAMESPACE).svc",
		},
	}

	testArgsToAppend := []string{
		"--test-arg", "--featureX=enable",
	}
	testArgsToOverrideReplace := []string{
		"--v=5", "--featureY=disable",
	}

	type TestData struct {
		deploymentName string
		overrides      *v1alpha1.UnsupportedConfigOverrides
		wantArgs       []string
	}
	tests := map[string]TestData{
		// unsupported config overrides as nil
		"nil config overrides should not touch the controller deployment": {
			deploymentName: "cert-manager",
			overrides:      nil,
			wantArgs:       defaultDeploymentArgs["cert-manager"],
		},
		"nil config overrides should not touch the cainjector deployment": {
			deploymentName: "cert-manager-cainjector",
			overrides:      nil,
			wantArgs:       defaultDeploymentArgs["cert-manager-cainjector"],
		},
		"nil config overrides should not touch the webhook deployment": {
			deploymentName: "cert-manager-webhook",
			overrides:      nil,
			wantArgs:       defaultDeploymentArgs["cert-manager-webhook"],
		},

		// unsupported config overrides as empty
		"Empty config overrides should not touch the controller deployment": {
			deploymentName: "cert-manager",
			overrides:      &v1alpha1.UnsupportedConfigOverrides{},
			wantArgs:       defaultDeploymentArgs["cert-manager"],
		},
		"Empty config overrides should not touch the cainjector deployment": {
			deploymentName: "cert-manager-cainjector",
			overrides:      &v1alpha1.UnsupportedConfigOverrides{},
			wantArgs:       defaultDeploymentArgs["cert-manager-cainjector"],
		},
		"Empty config overrides should not touch the webhook deployment": {
			deploymentName: "cert-manager-webhook",
			overrides:      &v1alpha1.UnsupportedConfigOverrides{},
			wantArgs:       defaultDeploymentArgs["cert-manager-webhook"],
		},

		// unsupported config overrides for webhook, cainjector should not
		// modify controller deployment
		"Other config overrides should not touch the controller deployment": {
			deploymentName: "cert-manager",
			overrides: &v1alpha1.UnsupportedConfigOverrides{
				Webhook: v1alpha1.UnsupportedConfigOverridesForCertManagerWebhook{
					Args: testArgsToAppend,
				},
				CAInjector: v1alpha1.UnsupportedConfigOverridesForCertManagerCAInjector{
					Args: testArgsToAppend,
				},
			},
			wantArgs: defaultDeploymentArgs["cert-manager"],
		},

		// unsupported config overrides as a mechanism of appending new args
		"Controller overrides should append newer overridden values": {
			deploymentName: "cert-manager",
			overrides: &v1alpha1.UnsupportedConfigOverrides{
				Controller: v1alpha1.UnsupportedConfigOverridesForCertManagerController{
					Args: testArgsToAppend,
				},
			},
			wantArgs: []string{
				"--acme-http01-solver-image=quay.io/jetstack/cert-manager-acmesolver:v1.18.3",
				"--cluster-resource-namespace=$(POD_NAMESPACE)",
				"--feature-gates=ACMEHTTP01IngressPathTypeExact=false",
				"--featureX=enable",
				"--leader-election-namespace=kube-system",
				"--max-concurrent-challenges=60",
				"--test-arg",
				"--v=2",
			},
		},
		"CAInjector overrides should append newer overridden values": {
			deploymentName: "cert-manager-cainjector",
			overrides: &v1alpha1.UnsupportedConfigOverrides{
				CAInjector: v1alpha1.UnsupportedConfigOverridesForCertManagerCAInjector{
					Args: testArgsToAppend,
				},
			},
			wantArgs: []string{
				"--featureX=enable",
				"--leader-election-namespace=kube-system",
				"--test-arg",
				"--v=2",
			},
		},
		"Webhook overrides should append newer overridden values": {
			deploymentName: "cert-manager-webhook",
			overrides: &v1alpha1.UnsupportedConfigOverrides{
				Webhook: v1alpha1.UnsupportedConfigOverridesForCertManagerWebhook{
					Args: testArgsToAppend,
				},
			},
			wantArgs: []string{
				"--dynamic-serving-ca-secret-name=cert-manager-webhook-ca",
				"--dynamic-serving-ca-secret-namespace=$(POD_NAMESPACE)",
				"--dynamic-serving-dns-names=cert-manager-webhook,cert-manager-webhook.$(POD_NAMESPACE),cert-manager-webhook.$(POD_NAMESPACE).svc",
				"--featureX=enable",
				"--secure-port=10250",
				"--test-arg",
				"--v=2",
			},
		},

		// unsupported config overrides as a mechanism of replacing existing values
		// of already present args
		"Controller overrides existing values for --v": {
			deploymentName: "cert-manager",
			overrides: &v1alpha1.UnsupportedConfigOverrides{
				Controller: v1alpha1.UnsupportedConfigOverridesForCertManagerController{
					Args: testArgsToOverrideReplace,
				},
			},
			wantArgs: []string{
				"--acme-http01-solver-image=quay.io/jetstack/cert-manager-acmesolver:v1.18.3",
				"--cluster-resource-namespace=$(POD_NAMESPACE)",
				"--feature-gates=ACMEHTTP01IngressPathTypeExact=false",
				"--featureY=disable",
				"--leader-election-namespace=kube-system",
				"--max-concurrent-challenges=60",
				"--v=5",
			},
		},
		"CAInjector overrides existing values for --v": {
			deploymentName: "cert-manager-cainjector",
			overrides: &v1alpha1.UnsupportedConfigOverrides{
				CAInjector: v1alpha1.UnsupportedConfigOverridesForCertManagerCAInjector{
					Args: testArgsToOverrideReplace,
				},
			},
			wantArgs: []string{
				"--featureY=disable",
				"--leader-election-namespace=kube-system",
				"--v=5",
			},
		},
		"Webhook overrides existing values for --v": {
			deploymentName: "cert-manager-webhook",
			overrides: &v1alpha1.UnsupportedConfigOverrides{
				Webhook: v1alpha1.UnsupportedConfigOverridesForCertManagerWebhook{
					Args: testArgsToOverrideReplace,
				},
			},
			wantArgs: []string{
				"--dynamic-serving-ca-secret-name=cert-manager-webhook-ca",
				"--dynamic-serving-ca-secret-namespace=$(POD_NAMESPACE)",
				"--dynamic-serving-dns-names=cert-manager-webhook,cert-manager-webhook.$(POD_NAMESPACE),cert-manager-webhook.$(POD_NAMESPACE).svc",
				"--featureY=disable",
				"--secure-port=10250",
				"--v=5",
			},
		},
	}

	for tcName, tcData := range tests {
		t.Run(tcName, func(t *testing.T) {
			t.Parallel()
			newDeployment := unsupportedConfigOverrides(deployments[tcData.deploymentName].DeepCopy(), tcData.overrides)
			require.Equal(t, tcData.wantArgs, newDeployment.Spec.Template.Spec.Containers[0].Args)
		})
	}
}

func TestParseEnvMap(t *testing.T) {
	env := mergeContainerEnvs([]corev1.EnvVar{
		{
			Name:  "A",
			Value: "asd",
		},
		{
			Name:  "B",
			Value: "32r23",
		},
	}, []corev1.EnvVar{
		{
			Name:  "A",
			Value: "23234",
		},
		{
			Name:  "C",
			Value: "a12sd",
		},
	})
	for _, e := range env {
		t.Logf("N: %s\t V:%s\n", e.Name, e.Value)
	}

	args := mergeContainerArgs([]string{"--a=12"}, []string{
		"A", "B", "--a=vc",
	})

	for _, s := range args {
		t.Logf("A:%q\n", s)
	}
}

func TestMergeContainerEnv(t *testing.T) {
	tests := []struct {
		name        string
		sourceEnv   []corev1.EnvVar
		overrideEnv []corev1.EnvVar
		expected    []corev1.EnvVar
	}{
		{
			name: "after merge, env values are sorted by key",
			sourceEnv: []corev1.EnvVar{
				{
					Name:  "XYZ",
					Value: "VALUE2",
				},
				{
					Name:  "ABC",
					Value: "VALUE1",
				},
			},
			overrideEnv: []corev1.EnvVar{
				{

					Name:  "DEF",
					Value: "VALUE1",
				},
			},
			expected: []corev1.EnvVar{
				{

					Name:  "ABC",
					Value: "VALUE1",
				},
				{
					Name:  "DEF",
					Value: "VALUE1",
				},
				{
					Name:  "XYZ",
					Value: "VALUE2",
				},
			},
		},
		{
			name: "override env replaces source env values",
			sourceEnv: []corev1.EnvVar{
				{
					Name:  "KEY2",
					Value: "VALUE2",
				},
				{
					Name:  "KEY1",
					Value: "VALUE1",
				},
			},
			overrideEnv: []corev1.EnvVar{
				{

					Name:  "KEY1",
					Value: "VALUE1",
				},
				{
					Name:  "KEY2",
					Value: "NEW_VALUE",
				},
			},
			expected: []corev1.EnvVar{
				{

					Name:  "KEY1",
					Value: "VALUE1",
				},
				{
					Name:  "KEY2",
					Value: "NEW_VALUE",
				},
			},
		},
	}

	for _, tc := range tests {
		actualEnv := mergeContainerEnvs(tc.sourceEnv, tc.overrideEnv)
		require.Equal(t, tc.expected, actualEnv)
	}
}

func TestMergeContainerArgs(t *testing.T) {
	tests := []struct {
		name         string
		sourceArgs   []string
		overrideArgs []string
		expected     []string
	}{
		{
			name:         "overrideargs replaces source arg values",
			sourceArgs:   []string{"--key1=value1", "--key2=value2"},
			overrideArgs: []string{"--key1=value1", "--key2=value5"},
			expected:     []string{"--key1=value1", "--key2=value5"},
		},
		{
			name:         "after merge, args are sorted in increasing order",
			sourceArgs:   []string{"--xxx1=value1", "--xyz=value2"},
			overrideArgs: []string{"--def=value1", "--abc=value5"},
			expected:     []string{"--abc=value5", "--def=value1", "--xxx1=value1", "--xyz=value2"},
		},
		{
			name:         "after merge, duplicates are removed",
			sourceArgs:   []string{"--abc=value1", "", "--xyz=value2"},
			overrideArgs: []string{"--xyz=value1", "--abc=value1"},
			expected:     []string{"--abc=value1", "--xyz=value1"},
		},
	}

	for _, tc := range tests {
		actualArgs := mergeContainerArgs(tc.sourceArgs, tc.overrideArgs)
		require.Equal(t, tc.expected, actualArgs)
	}
}

func TestParseArgMap(t *testing.T) {
	testArgs := []string{
		"", // should be ignored at the time of parse
		"--", "--foo", "--v=1", "--test=v1=v2", "--gates=Feature1=True",
		"--log-level=Debug=false,Info=false,Warning=True,Error=true",
		"--extra-flags='--v=2 --gates=Feature2=True'",
	}
	wantMap := map[string]string{
		"--":            "",
		"--foo":         "",
		"--v":           "1",
		"--test":        "v1=v2",
		"--gates":       "Feature1=True",
		"--log-level":   "Debug=false,Info=false,Warning=True,Error=true",
		"--extra-flags": "'--v=2 --gates=Feature2=True'",
	}

	argMap := make(map[string]string)
	parseArgMap(argMap, testArgs)
	if !reflect.DeepEqual(argMap, wantMap) {
		t.Fatalf("unexpected update to arg map, diff = %v", cmp.Diff(wantMap, argMap))
	}
}

// newTestCertManagerInformer sets up a CertManager informer backed by a fake
// clientset and returns the informer along with a function that creates a
// CertManager object and blocks until the informer's cache observes it.
func newTestCertManagerInformer(t *testing.T, ctx context.Context) (certmanagerinformer.CertManagerInformer, func(*v1alpha1.CertManager)) {
	t.Helper()

	watcherStarted := make(chan struct{})
	fakeClient := fake.NewSimpleClientset()
	fakeClient.PrependWatchReactor("certmanagers", func(action clienttesting.Action) (handled bool, ret watch.Interface, err error) {
		gvr := action.GetResource()
		ns := action.GetNamespace()
		w, err := fakeClient.Tracker().Watch(gvr, ns)
		if err != nil {
			return false, nil, err
		}
		close(watcherStarted)
		return true, w, nil
	})

	informer := certmanoperatorinformer.NewSharedInformerFactory(fakeClient, 0).Operator().V1alpha1().CertManagers()

	certManagerChan := make(chan *v1alpha1.CertManager, 1)
	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			certManagerChan <- obj.(*v1alpha1.CertManager)
		},
	})

	go informer.Informer().Run(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), informer.Informer().HasSynced)
	<-watcherStarted

	create := func(cm *v1alpha1.CertManager) {
		t.Helper()
		_, err := fakeClient.OperatorV1alpha1().CertManagers().Create(ctx, cm, metav1.CreateOptions{})
		require.NoError(t, err)
		select {
		case <-certManagerChan:
		case <-time.After(wait.ForeverTestTimeout):
			t.Fatal("informer did not observe the created cert manager object")
		}
	}

	return informer, create
}

// TestProxyEnvOverridePrecedence verifies that user-specified proxy values in
// spec.controllerConfig.overrideEnv take precedence over the cluster-wide proxy
// settings injected by withProxyEnv. This guards the hook ordering in
// newGenericDeploymentController where withProxyEnv must run before
// withContainerEnvOverrideHook so that the user override is applied last.
func TestProxyEnvOverridePrecedence(t *testing.T) {
	// Simulate cluster-wide proxy settings read from the operator environment.
	t.Setenv("HTTP_PROXY", "http://cluster-proxy.example.com:3128")
	t.Setenv("HTTPS_PROXY", "https://cluster-proxy.example.com:3128")
	t.Setenv("NO_PROXY", "cluster-no-proxy")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	informer, create := newTestCertManagerInformer(t, ctx)

	// User configures custom proxy values via overrideEnv.
	userEnv := []corev1.EnvVar{
		{Name: "HTTP_PROXY", Value: "http://custom-proxy.example.com:8080"},
		{Name: "HTTPS_PROXY", Value: "https://custom-proxy.example.com:8080"},
		{Name: "NO_PROXY", Value: "localhost,127.0.0.1"},
	}
	create(&v1alpha1.CertManager{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: v1alpha1.CertManagerSpec{
			ControllerConfig: &v1alpha1.DeploymentConfig{
				OverrideEnv: userEnv,
			},
		},
	})

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: certmanagerControllerDeployment},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: certmanagerControllerDeployment}},
				},
			},
		},
	}

	// Apply the hooks in the same order used by newGenericDeploymentController:
	// cluster proxy first, user overrideEnv last so it wins.
	require.NoError(t, withProxyEnv(nil, deployment))
	require.NoError(t, withContainerEnvOverrideHook(informer, certmanagerControllerDeployment, getOverrideEnvFor)(nil, deployment))

	got := map[string]string{}
	for _, env := range deployment.Spec.Template.Spec.Containers[0].Env {
		got[env.Name] = env.Value
	}

	for _, env := range userEnv {
		require.Equalf(t, env.Value, got[env.Name],
			"user-specified overrideEnv %q should take precedence over the cluster proxy value", env.Name)
	}
}
