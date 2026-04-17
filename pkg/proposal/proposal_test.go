package proposal

import (
	"context"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"
)

func newFakeClient(objects ...runtime.Object) *dynamicfake.FakeDynamicClient {
	scheme := runtime.NewScheme()
	gvr := schema.GroupVersionResource{Group: "ols.openshift.io", Version: "v1alpha1", Resource: "lightspeedproposals"}
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: "ols.openshift.io", Version: "v1alpha1", Kind: "LightspeedProposal"},
		&unstructured.Unstructured{},
	)
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: "ols.openshift.io", Version: "v1alpha1", Kind: "LightspeedProposalList"},
		&unstructured.UnstructuredList{},
	)
	cmGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
		&unstructured.Unstructured{},
	)
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMapList"},
		&unstructured.UnstructuredList{},
	)
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			gvr:   "LightspeedProposalList",
			cmGVR: "ConfigMapList",
		}, objects...)
}

func TestAlertProposalName(t *testing.T) {
	tests := []struct {
		alertName string
		namespace string
	}{
		{"PrometheusBadConfig", "openshift-monitoring"},
		{"AlertmanagerFailedReload", "openshift-monitoring"},
		{"TargetDown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.alertName, func(t *testing.T) {
			name := alertProposalName(tt.alertName, tt.namespace)
			if !strings.HasPrefix(name, "cmo-alert-") {
				t.Errorf("alertProposalName() = %q, want prefix cmo-alert-", name)
			}
			// Deterministic: same input produces same name
			name2 := alertProposalName(tt.alertName, tt.namespace)
			if name != name2 {
				t.Errorf("alertProposalName() not deterministic: %q != %q", name, name2)
			}
		})
	}
}

func TestAlertProposalName_DifferentNamespacesDifferentNames(t *testing.T) {
	name1 := alertProposalName("TargetDown", "ns-a")
	name2 := alertProposalName("TargetDown", "ns-b")
	if name1 == name2 {
		t.Errorf("same alert in different namespaces should produce different names: %q", name1)
	}
}

func TestDedupHash(t *testing.T) {
	tests := []struct {
		name  string
		parts []string
	}{
		{"single", []string{"PrometheusBadConfig"}},
		{"two parts", []string{"PrometheusBadConfig", "openshift-monitoring"}},
		{"three parts", []string{"a", "b", "c"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := dedupHash(tt.parts...)
			if len(hash) != 16 { // 8 bytes = 16 hex chars
				t.Errorf("dedupHash() = %q, want 16 hex chars", hash)
			}
			// Deterministic
			hash2 := dedupHash(tt.parts...)
			if hash != hash2 {
				t.Errorf("dedupHash() not deterministic: %q != %q", hash, hash2)
			}
		})
	}

	// Different inputs produce different hashes
	h1 := dedupHash("a", "b")
	h2 := dedupHash("a", "c")
	if h1 == h2 {
		t.Errorf("different inputs should produce different hashes")
	}
}

func TestSanitize(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"PrometheusBadConfig", "prometheusbadconfig"},
		{"AlertmanagerFailedReload", "alertmanagerfailedre"},
		{"target.down", "target-down"},
		{"hello world", "hello-world"},
		{"snake_case", "snake-case"},
		{"trailing-dot.", "trailing-dot"},
		{"trailing-dash-", "trailing-dash"},
		{"a-very-long-version-string-that-is-too-long", "a-very-long-version"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitize(tt.input)
			if got != tt.expected {
				t.Errorf("sanitize(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestBuildAlertRequest(t *testing.T) {
	t.Run("full alert", func(t *testing.T) {
		alert := AlertInfo{
			Name:        "PrometheusBadConfig",
			Severity:    "critical",
			Namespace:   "openshift-monitoring",
			Pod:         "prometheus-k8s-0",
			Description: "Prometheus failed to reload its configuration.",
			RunbookURL:  "https://runbooks.example.com/PrometheusBadConfig",
			Labels: map[string]string{
				"job": "prometheus-k8s",
			},
		}

		request := buildAlertRequest("System prompt here.", alert)

		if !strings.Contains(request, "System prompt here.") {
			t.Error("request should contain system prompt")
		}
		if !strings.Contains(request, "Alert: PrometheusBadConfig") {
			t.Error("request should contain alert name")
		}
		if !strings.Contains(request, "Severity: critical") {
			t.Error("request should contain severity")
		}
		if !strings.Contains(request, "Namespace: openshift-monitoring") {
			t.Error("request should contain namespace")
		}
		if !strings.Contains(request, "Pod: prometheus-k8s-0") {
			t.Error("request should contain pod")
		}
		if !strings.Contains(request, "## Description") {
			t.Error("request should contain description header")
		}
		if !strings.Contains(request, "## Runbook") {
			t.Error("request should contain runbook header")
		}
		if !strings.Contains(request, "## Alert Labels") {
			t.Error("request should contain labels header")
		}
	})

	t.Run("minimal alert", func(t *testing.T) {
		alert := AlertInfo{
			Name:     "TargetDown",
			Severity: "warning",
		}

		request := buildAlertRequest("", alert)

		if strings.Contains(request, "---") {
			t.Error("no system prompt should not have separator")
		}
		if !strings.Contains(request, "Alert: TargetDown") {
			t.Error("request should contain alert name")
		}
		if strings.Contains(request, "Namespace:") {
			t.Error("empty namespace should be omitted")
		}
		if strings.Contains(request, "Pod:") {
			t.Error("empty pod should be omitted")
		}
	})
}

func TestMaybeCreateAlertProposal_CreatesNew(t *testing.T) {
	client := newFakeClient()
	creator := NewCreator(client, Config{
		Namespace:     "test-ns",
		AlertWorkflow: "cmo-alert-remediation",
	})

	alert := AlertInfo{
		Name:     "PrometheusBadConfig",
		Severity: "critical",
	}

	creator.MaybeCreateAlertProposal(context.Background(), alert)

	var createAction *clienttesting.CreateActionImpl
	for _, action := range client.Actions() {
		if action.GetVerb() == "create" && action.GetResource().Resource == "lightspeedproposals" {
			a := action.(clienttesting.CreateAction)
			ca := clienttesting.CreateActionImpl{
				ActionImpl: clienttesting.ActionImpl{Verb: "create", Resource: a.GetResource(), Namespace: a.GetNamespace()},
				Object:     a.GetObject(),
			}
			createAction = &ca
			break
		}
	}

	if createAction == nil {
		t.Fatal("expected a create action for lightspeedproposals")
	}

	obj := createAction.Object.(*unstructured.Unstructured)

	// Verify deterministic name
	expectedName := alertProposalName("PrometheusBadConfig", "")
	if got := obj.GetName(); got != expectedName {
		t.Errorf("proposal name = %q, want %q", got, expectedName)
	}
	if got := obj.GetNamespace(); got != "test-ns" {
		t.Errorf("proposal namespace = %q, want %q", got, "test-ns")
	}

	labels := obj.GetLabels()
	if got := labels["ols.openshift.io/source"]; got != "cluster-monitoring-operator" {
		t.Errorf("source label = %q, want %q", got, "cluster-monitoring-operator")
	}
	if got := labels["ols.openshift.io/alertname"]; got != sanitize("PrometheusBadConfig") {
		t.Errorf("alertname label = %q, want %q", got, sanitize("PrometheusBadConfig"))
	}
	if got := labels["ols.openshift.io/severity"]; got != "critical" {
		t.Errorf("severity label = %q, want %q", got, "critical")
	}

	workflow, _, _ := unstructured.NestedString(obj.Object, "spec", "workflow")
	if workflow != "cmo-alert-remediation" {
		t.Errorf("spec.workflow = %q, want %q", workflow, "cmo-alert-remediation")
	}
}

func TestMaybeCreateAlertProposal_DedupSkipsExisting(t *testing.T) {
	alert := AlertInfo{
		Name:      "PrometheusBadConfig",
		Severity:  "critical",
		Namespace: "openshift-monitoring",
	}

	existing := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "ols.openshift.io/v1alpha1",
			"kind":       "LightspeedProposal",
			"metadata": map[string]interface{}{
				"name":      alertProposalName(alert.Name, alert.Namespace),
				"namespace": "test-ns",
			},
		},
	}

	client := newFakeClient(existing)
	creator := NewCreator(client, Config{
		Namespace:     "test-ns",
		AlertWorkflow: "cmo-alert-remediation",
	})

	creator.MaybeCreateAlertProposal(context.Background(), alert)

	for _, action := range client.Actions() {
		if action.GetVerb() == "create" && action.GetResource().Resource == "lightspeedproposals" {
			t.Error("should not create proposal when one already exists")
		}
	}
}

func TestMaybeCreateAlertProposal_WithSystemPrompt(t *testing.T) {
	promptCM := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      "cmo-analysis-prompt",
				"namespace": "test-ns",
			},
			"data": map[string]interface{}{
				"prompt": "You are a monitoring specialist.",
			},
		},
	}

	client := newFakeClient(promptCM)
	creator := NewCreator(client, Config{
		Namespace:       "test-ns",
		AlertWorkflow:   "cmo-alert-remediation",
		PromptConfigMap: "cmo-analysis-prompt",
	})

	alert := AlertInfo{
		Name:     "TargetDown",
		Severity: "warning",
	}

	creator.MaybeCreateAlertProposal(context.Background(), alert)

	for _, action := range client.Actions() {
		if action.GetVerb() == "create" && action.GetResource().Resource == "lightspeedproposals" {
			obj := action.(clienttesting.CreateAction).GetObject().(*unstructured.Unstructured)
			request, _, _ := unstructured.NestedString(obj.Object, "spec", "request")
			if !strings.Contains(request, "You are a monitoring specialist.") {
				t.Error("request should contain system prompt from ConfigMap")
			}
			return
		}
	}

	t.Fatal("expected a create action with system prompt")
}
