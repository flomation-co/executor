// Package infrastructure_kubernetes_deployment_rollout_status reports whether a
// Deployment's rollout has finished, exactly as `kubectl rollout status` does.
//
// There is no rollout-status endpoint: kubectl derives the verdict from fields
// on the Deployment object, and so does this. The rules, in order, are the ones
// deploymentutil applies:
//
//   - spec.generation not yet observed  -> the controller hasn't seen the latest
//     edit; still waiting.
//   - a Progressing condition with reason ProgressDeadlineExceeded -> FAILED; the
//     rollout will not finish on its own.
//   - fewer updatedReplicas than desired -> new pods still rolling out.
//   - more replicas than updatedReplicas -> old pods still terminating.
//   - fewer availableReplicas than updatedReplicas -> new pods not yet ready.
//   - otherwise -> rolled out.
//
// Every numeric status field is absent, not zero, on a fresh object; both read
// the same through nestedInt, which treats a missing field as 0.
//
// With Wait set the action polls every 2s until the rollout completes, fails, or
// timeout_seconds elapses. It runs that poll under its OWN context, bounded at
// timeout_seconds + 30s, rather than kubernetes.Context(): a wait can legitimately
// outlast a single request's deadline, and the per-request HTTP timeout still
// bounds each individual Get.
package infrastructure_kubernetes_deployment_rollout_status

import (
	"context"
	"fmt"
	"time"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/kubernetes"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "Kubernetes Deployment Rollout Status"
	Description  = "Check whether a Deployment has finished rolling out — optionally waiting until it completes, fails, or times out."
	Website      = "https://www.flomation.co"
	Icon         = "kubernetes+check"
	Date         = "10/07/2026"
	Type         = core.ActionTypeAction
)

// defaultTimeoutSeconds mirrors kubectl's default rollout-status wait.
const defaultTimeoutSeconds = 300

var Inputs = [...]core.Connection{
	{Name: "api_server_url", Type: core.ConnectionTypeString, Label: "API Server URL", Placeholder: "https://your-cluster:6443 — the Kubernetes API endpoint", Required: true},
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{
		{Name: "Service Account Token", Value: "token"},
		{Name: "Client Certificate (mTLS)", Value: "client_cert"},
		{Name: "Kubeconfig (paste)", Value: "kubeconfig"},
	}},
	{Name: "service_account_token", Type: core.ConnectionTypeSecret, Label: "Service Account Token", Placeholder: "kubectl create token <serviceaccount> -n <namespace>", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "token"}}},
	{Name: "cluster_ca_cert", Type: core.ConnectionTypeText, Label: "Cluster CA Certificate (PEM)", Placeholder: "-----BEGIN CERTIFICATE----- … Leave blank to use the system trust store", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "token", "client_cert"}}},
	{Name: "client_certificate", Type: core.ConnectionTypeSecret, Label: "Client Certificate (PEM)", Placeholder: "-----BEGIN CERTIFICATE-----", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"client_cert"}}},
	{Name: "client_key", Type: core.ConnectionTypeSecret, Label: "Client Key (PEM)", Placeholder: "-----BEGIN RSA PRIVATE KEY-----", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"client_cert"}}},
	{Name: "kubeconfig", Type: core.ConnectionTypeSecret, Label: "Kubeconfig YAML", Placeholder: "Paste the full kubeconfig; the current-context is used", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"kubeconfig"}}},
	{Name: "allow_insecure", Type: core.ConnectionTypeBoolean, Label: "Allow Insecure TLS", Placeholder: "Skip API server certificate verification — only for self-signed clusters with no CA to hand"},

	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace", Placeholder: "The namespace the deployment lives in", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Deployment", Placeholder: "The deployment to check", Required: true},
	{Name: "wait", Type: core.ConnectionTypeBoolean, Label: "Wait Until Complete", Placeholder: "Poll every 2s until the rollout finishes, fails, or times out"},
	{Name: "timeout_seconds", Type: core.ConnectionTypeInteger, Label: "Timeout (seconds)", Placeholder: "How long to wait when Wait is set (default 300)"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Deployment Name"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Deployment"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status"},
	{Name: "complete", Type: core.ConnectionTypeBoolean, Label: "Complete"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := kubernetes.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	namespace, err := kubernetes.RequiredString("namespace", inputs)
	if err != nil {
		return nil, err
	}
	name, err := kubernetes.RequiredString("name", inputs)
	if err != nil {
		return nil, err
	}

	wait := kubernetes.BoolInput("wait", inputs)

	// One-shot: read the object once and report where the rollout stands.
	if !wait {
		ctx, cancel := kubernetes.Context()
		defer cancel()

		obj, err := kubernetes.Get(ctx, auth, kubernetes.Deployments, namespace, name)
		if err != nil {
			return kubernetes.ErrorResult(err.Error()), nil
		}
		complete, failed, status := rolloutStatus(obj, name)
		return result(obj, name, status, complete, failed), nil
	}

	timeoutSeconds := defaultTimeoutSeconds
	if t, ok := kubernetes.OptionalInt("timeout_seconds", inputs); ok && t > 0 {
		timeoutSeconds = t
	}
	deadline := time.Duration(timeoutSeconds) * time.Second

	// The wait's own context outlives a single request timeout, so a slow API
	// server does not cut the poll short before timeout_seconds is up.
	ctx, cancel := context.WithTimeout(context.Background(), deadline+30*time.Second)
	defer cancel()

	start := time.Now()
	var lastObj map[string]interface{}
	var lastStatus string
	for {
		obj, err := kubernetes.Get(ctx, auth, kubernetes.Deployments, namespace, name)
		if err != nil {
			return kubernetes.ErrorResult(err.Error()), nil
		}
		lastObj = obj
		complete, failed, status := rolloutStatus(obj, name)
		lastStatus = status
		if complete || failed {
			return result(obj, name, status, complete, failed), nil
		}
		if time.Since(start) >= deadline {
			return timedOut(lastObj, name, timeoutSeconds, lastStatus), nil
		}

		select {
		case <-ctx.Done():
			return timedOut(lastObj, name, timeoutSeconds, lastStatus), nil
		case <-time.After(2 * time.Second):
		}
	}
}

// rolloutStatus applies kubectl's rollout-status rules to a decoded Deployment,
// returning whether the rollout is complete, whether it has failed outright, and
// a human-readable status line.
func rolloutStatus(obj map[string]interface{}, name string) (complete, failed bool, status string) {
	generation := nestedInt(obj, "metadata", "generation")
	observed := nestedInt(obj, "status", "observedGeneration")
	if generation > observed {
		return false, false, "Waiting for deployment spec update to be observed"
	}

	if progressDeadlineExceeded(obj) {
		return false, true, fmt.Sprintf("deployment %q exceeded its progress deadline", name)
	}

	specReplicas := nestedInt(obj, "spec", "replicas")
	updated := nestedInt(obj, "status", "updatedReplicas")
	if updated < specReplicas {
		return false, false, fmt.Sprintf("Waiting for deployment %q rollout to finish: %d out of %d new replicas have been updated", name, updated, specReplicas)
	}

	replicas := nestedInt(obj, "status", "replicas")
	if replicas > updated {
		return false, false, fmt.Sprintf("Waiting for deployment %q rollout to finish: %d old replicas are pending termination", name, replicas-updated)
	}

	available := nestedInt(obj, "status", "availableReplicas")
	if available < updated {
		return false, false, fmt.Sprintf("Waiting for deployment %q rollout to finish: %d of %d updated replicas are available", name, available, updated)
	}

	return true, false, fmt.Sprintf("deployment %q successfully rolled out", name)
}

// progressDeadlineExceeded reports whether the Deployment's Progressing condition
// has stalled with reason ProgressDeadlineExceeded — kubectl's TimedOutReason.
func progressDeadlineExceeded(obj map[string]interface{}) bool {
	st, ok := obj["status"].(map[string]interface{})
	if !ok {
		return false
	}
	conds, ok := st["conditions"].([]interface{})
	if !ok {
		return false
	}
	for _, c := range conds {
		cm, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		t, _ := cm["type"].(string)
		r, _ := cm["reason"].(string)
		if t == "Progressing" && r == "ProgressDeadlineExceeded" {
			return true
		}
	}
	return false
}

// nestedInt walks a decoded object down a key path and returns the leaf as an
// int, treating a missing field (or any non-numeric leaf) as 0. JSON numbers
// decode as float64, which is the case that matters here.
func nestedInt(obj map[string]interface{}, keys ...string) int {
	var cur interface{} = obj
	for _, k := range keys {
		m, ok := cur.(map[string]interface{})
		if !ok {
			return 0
		}
		cur, ok = m[k]
		if !ok {
			return 0
		}
	}
	switch v := cur.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	default:
		return 0
	}
}

// result builds the standard output map. A failed rollout reports success=false
// so the flow's error port fires; an in-progress rollout is a legitimate read,
// not an error, so success stays true with an empty error string.
func result(obj map[string]interface{}, name, status string, complete, failed bool) map[string]interface{} {
	errMsg := ""
	if failed {
		errMsg = status
	}
	return map[string]interface{}{
		"id":          name,
		"result":      obj,
		"status":      status,
		"complete":    complete,
		"tool_result": status,
		"success":     !failed,
		"error":       errMsg,
	}
}

// timedOut is the soft failure returned when a wait exhausts its budget. It is an
// ErrorResult carrying the last-seen object and status, so the error port fires
// but a caller can still inspect how far the rollout got.
func timedOut(obj map[string]interface{}, name string, timeoutSeconds int, lastStatus string) map[string]interface{} {
	msg := fmt.Sprintf("Timed out after %ds waiting for deployment %s to roll out — last status: %s", timeoutSeconds, name, lastStatus)
	out := kubernetes.ErrorResult(msg)
	out["id"] = name
	out["result"] = obj
	out["status"] = lastStatus
	out["complete"] = false
	return out
}
