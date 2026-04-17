package proposal

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
)

var lightspeedProposalGVR = schema.GroupVersionResource{
	Group:    "ols.openshift.io",
	Version:  "v1alpha1",
	Resource: "lightspeedproposals",
}

const DefaultAlertWorkflow = "cmo-alert-remediation"

var configMapGVR = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}

// Config holds configuration for proposal creation.
type Config struct {
	Namespace       string
	AlertWorkflow   string
	PromptConfigMap string
}

// DefaultConfig returns the default configuration, checking env vars for overrides.
func DefaultConfig() Config {
	return Config{
		Namespace:       envOrDefault("LIGHTSPEED_PROPOSAL_NAMESPACE", "openshift-lightspeed"),
		AlertWorkflow:   envOrDefault("LIGHTSPEED_ALERT_WORKFLOW", DefaultAlertWorkflow),
		PromptConfigMap: envOrDefault("LIGHTSPEED_PROMPT_CONFIGMAP", "cmo-analysis-prompt"),
	}
}

// AlertInfo describes a firing Prometheus alert.
type AlertInfo struct {
	Name        string
	Severity    string
	Namespace   string
	Pod         string
	Container   string
	Description string
	RunbookURL  string
	Labels      map[string]string
}

// Creator creates LightspeedProposal CRs for monitoring events.
type Creator struct {
	client dynamic.Interface
	config Config
}

// NewCreator returns a new proposal Creator.
func NewCreator(client dynamic.Interface, config Config) *Creator {
	return &Creator{
		client: client,
		config: config,
	}
}

// MaybeCreateAlertProposal creates a proposal for a firing alert if one doesn't already exist.
// Errors are logged but never returned — proposal creation must never block CMO.
func (c *Creator) MaybeCreateAlertProposal(ctx context.Context, alert AlertInfo) {
	name := alertProposalName(alert.Name, alert.Namespace)

	_, err := c.client.Resource(lightspeedProposalGVR).Namespace(c.config.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		klog.V(4).Infof("LightspeedProposal %s/%s already exists, skipping", c.config.Namespace, name)
		return
	}
	if !errors.IsNotFound(err) {
		if isNoMatchError(err) {
			klog.V(4).Infof("LightspeedProposal CRD not found, skipping proposal creation")
			return
		}
		klog.Warningf("Failed to check LightspeedProposal %s/%s: %v", c.config.Namespace, name, err)
		return
	}

	systemPrompt := c.readSystemPrompt(ctx, c.config.PromptConfigMap)
	request := buildAlertRequest(systemPrompt, alert)

	if err := c.createProposal(ctx, name, c.config.AlertWorkflow, request, map[string]interface{}{
		"ols.openshift.io/source":    "cluster-monitoring-operator",
		"ols.openshift.io/alertname": sanitize(alert.Name),
		"ols.openshift.io/severity":  alert.Severity,
		"ols.openshift.io/dedup-key": dedupHash(alert.Name, alert.Namespace),
	}); err != nil {
		if isNoMatchError(err) {
			klog.V(4).Infof("LightspeedProposal CRD not found, skipping proposal creation")
			return
		}
		klog.Warningf("Failed to create LightspeedProposal %s/%s: %v", c.config.Namespace, name, err)
		return
	}

	klog.Infof("Created LightspeedProposal %s/%s for alert %s (severity=%s)",
		c.config.Namespace, name, alert.Name, alert.Severity)
}


// MaybeCreateTriggerProposal creates a bootstrap proposal for a custom trigger.
// The proposal's request contains the plain English intent. The agent will
// discover metrics, write PromQL, and create a PrometheusRule during execution.
// Errors are logged but never returned.
func (c *Creator) MaybeCreateTriggerProposal(triggerName, intent, workflow string) {
	name := fmt.Sprintf("cmo-trigger-%s", sanitize(triggerName))

	ctx := context.Background()
	_, err := c.client.Resource(lightspeedProposalGVR).Namespace(c.config.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		klog.V(4).Infof("LightspeedProposal %s/%s already exists (trigger bootstrap), skipping", c.config.Namespace, name)
		return
	}
	if !errors.IsNotFound(err) {
		if isNoMatchError(err) {
			klog.V(4).Infof("LightspeedProposal CRD not found, skipping trigger bootstrap")
			return
		}
		klog.Warningf("Failed to check LightspeedProposal %s/%s: %v", c.config.Namespace, name, err)
		return
	}

	systemPrompt := c.readSystemPrompt(ctx, c.config.PromptConfigMap)

	var b strings.Builder
	if systemPrompt != "" {
		b.WriteString(systemPrompt)
		b.WriteString("\n\n---\n\n")
	}
	fmt.Fprintf(&b, "## Custom Trigger Bootstrap\n\n")
	fmt.Fprintf(&b, "Trigger name: %s\n\n", triggerName)
	fmt.Fprintf(&b, "## User Intent\n\n%s\n\n", intent)
	b.WriteString("## Instructions\n\n")
	b.WriteString("1. Discover relevant Prometheus metrics for this intent.\n")
	b.WriteString("2. Write one or more PromQL expressions that detect the condition.\n")
	b.WriteString("3. Propose multiple remediation approaches as options.\n")
	b.WriteString("4. For each option, include the PrometheusRule YAML and remediation strategy.\n")
	b.WriteString("5. After the user selects an option, create the PrometheusRule CR in openshift-monitoring.\n")

	if err := c.createProposal(ctx, name, workflow, b.String(), map[string]interface{}{
		"ols.openshift.io/source":        "cluster-monitoring-operator",
		"ols.openshift.io/proposal-type": "trigger-bootstrap",
		"ols.openshift.io/trigger-name":  sanitize(triggerName),
	}); err != nil {
		if isNoMatchError(err) {
			klog.V(4).Infof("LightspeedProposal CRD not found, skipping trigger bootstrap")
			return
		}
		klog.Warningf("Failed to create LightspeedProposal %s/%s: %v", c.config.Namespace, name, err)
		return
	}

	klog.Infof("Created trigger bootstrap LightspeedProposal %s/%s for intent %q", c.config.Namespace, name, triggerName)
}

func (c *Creator) readSystemPrompt(ctx context.Context, configMapName string) string {
	if configMapName == "" {
		return ""
	}
	obj, err := c.client.Resource(configMapGVR).Namespace(c.config.Namespace).Get(ctx, configMapName, metav1.GetOptions{})
	if err != nil {
		klog.V(4).Infof("Could not read system prompt ConfigMap %s/%s: %v", c.config.Namespace, configMapName, err)
		return ""
	}
	data, _, _ := unstructured.NestedMap(obj.Object, "data")
	if prompt, ok := data["prompt"].(string); ok {
		return prompt
	}
	return ""
}

func buildAlertRequest(systemPrompt string, alert AlertInfo) string {
	var b strings.Builder

	if systemPrompt != "" {
		b.WriteString(systemPrompt)
		b.WriteString("\n\n---\n\n")
	}

	fmt.Fprintf(&b, "Alert: %s\n", alert.Name)
	fmt.Fprintf(&b, "Severity: %s\n", alert.Severity)
	if alert.Namespace != "" {
		fmt.Fprintf(&b, "Namespace: %s\n", alert.Namespace)
	}
	if alert.Pod != "" {
		fmt.Fprintf(&b, "Pod: %s\n", alert.Pod)
	}
	if alert.Container != "" {
		fmt.Fprintf(&b, "Container: %s\n", alert.Container)
	}
	b.WriteString("\n")

	if alert.Description != "" {
		fmt.Fprintf(&b, "## Description\n\n%s\n\n", alert.Description)
	}

	if alert.RunbookURL != "" {
		fmt.Fprintf(&b, "## Runbook\n\n%s\n\n", alert.RunbookURL)
	}

	if len(alert.Labels) > 0 {
		b.WriteString("## Alert Labels\n\n")
		for k, v := range alert.Labels {
			fmt.Fprintf(&b, "- %s: %s\n", k, v)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// alertProposalName generates a deterministic name for an alert proposal.
func alertProposalName(alertName, namespace string) string {
	return fmt.Sprintf("cmo-alert-%s-%s", sanitize(alertName), dedupHash(alertName, namespace))
}

// sanitize converts a string into a valid DNS-1035 label component.
func sanitize(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, ".", "-")
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	if len(s) > 20 {
		s = s[:20]
	}
	return strings.TrimRight(s, "-")
}

// dedupHash produces a short deterministic hash for deduplication.
func dedupHash(parts ...string) string {
	key := strings.Join(parts, "/")
	h := sha256.Sum256([]byte(key))
	return fmt.Sprintf("%x", h[:8])
}

func (c *Creator) createProposal(ctx context.Context, name, workflow, request string, labels map[string]interface{}) error {
	proposal := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "ols.openshift.io/v1alpha1",
			"kind":       "LightspeedProposal",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": c.config.Namespace,
				"labels":    labels,
			},
			"spec": map[string]interface{}{
				"workflow":    workflow,
				"request":     request,
				"maxAttempts": int64(2),
			},
		},
	}
	_, err := c.client.Resource(lightspeedProposalGVR).Namespace(c.config.Namespace).Create(ctx, proposal, metav1.CreateOptions{})
	return err
}

func isNoMatchError(err error) bool {
	return meta.IsNoMatchError(err) || errors.IsNotFound(err)
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
