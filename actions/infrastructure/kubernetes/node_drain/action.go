// Package infrastructure_kubernetes_node_drain safely evacuates a Node, as
// `kubectl drain` does: cordon it, then evict every eligible pod so the workloads
// reschedule elsewhere before the node is taken down for maintenance.
//
// There is no single "drain" verb in the Kubernetes API — kubectl composes it
// from a cordon patch, a pod list, and one Eviction request per pod, honouring
// PodDisruptionBudgets along the way. This action reproduces that composition:
//
//   - Cordon FIRST (spec.unschedulable=true). If the eviction loop later fails,
//     the node is deliberately LEFT cordoned — that is kubectl's behaviour, and
//     un-cordoning a half-drained node would let the scheduler refill it with the
//     very pods we are trying to move off.
//   - Classify each pod on the node. Mirror pods (static, kubelet-owned) and pods
//     already terminating are skipped; DaemonSet pods are skipped or, if the
//     operator did not opt into ignoring them, block the drain; a pod with an
//     emptyDir volume blocks unless the operator accepts the data loss; an
//     unmanaged pod (no ownerReferences) blocks because nothing would recreate it.
//     If anything blocks, we abort BEFORE evicting a single pod, leaving the node
//     cordoned and every pod where it was.
//   - Evict the rest via the policy/v1 Eviction subresource, which is what makes a
//     PodDisruptionBudget able to say "not yet" (HTTP 429). A 429 is retried with
//     capped backoff until the timeout; a 404 means the pod already left and is
//     treated as success.
//
// This is destructive — it moves running workloads — so it is gated on
// confirm_destructive, which fails closed (see kubernetes.ConfirmDestructive).
package infrastructure_kubernetes_node_drain

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/kubernetes"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "Kubernetes Drain Node"
	Description  = "Cordon a Node and evict its pods so they reschedule elsewhere, ready for maintenance. Requires explicit confirmation."
	Website      = "https://www.flomation.co"
	Icon         = "kubernetes+arrow-down"
	Date         = "10/07/2026"
	Type         = core.ActionTypeAction
)

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

	{Name: "name", Type: core.ConnectionTypeString, Label: "Node", Placeholder: "The node to drain", Required: true},
	{Name: "grace_period_seconds", Type: core.ConnectionTypeInteger, Label: "Grace Period (seconds)", Placeholder: "Override each pod's termination grace period; blank to use the pod's own"},
	{Name: "ignore_daemonsets", Type: core.ConnectionTypeBoolean, Label: "Ignore DaemonSets", Placeholder: "Skip DaemonSet-managed pods rather than blocking on them (default: on)"},
	{Name: "delete_emptydir_data", Type: core.ConnectionTypeBoolean, Label: "Delete emptyDir Data", Placeholder: "Allow evicting pods that use emptyDir volumes — their data is lost (default: off)"},
	{Name: "timeout_seconds", Type: core.ConnectionTypeInteger, Label: "Timeout (seconds)", Placeholder: "Give up if the drain has not completed within this many seconds (default 300)"},
	{Name: "confirm_destructive", Type: core.ConnectionTypeBoolean, Label: "Confirm Destructive Action", Placeholder: "This evicts running pods off the node. Tick to allow, or bind a variable such as ${var.approved}", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Node Name"},
	{Name: "evicted", Type: core.ConnectionTypeInteger, Label: "Evicted"},
	{Name: "skipped", Type: core.ConnectionTypeInteger, Label: "Skipped"},
	{Name: "evicted_pods", Type: core.ConnectionTypeObject, Label: "Evicted Pods"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// defaultTimeoutSeconds bounds the whole drain when the operator leaves the
// Timeout field blank, matching kubectl's own default posture of not waiting
// forever on a PodDisruptionBudget that never lets go.
const defaultTimeoutSeconds = 300

// podRef is a pod addressed for eviction.
type podRef struct{ namespace, name string }

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := kubernetes.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	name, err := kubernetes.RequiredString("name", inputs)
	if err != nil {
		return nil, err
	}
	if err := kubernetes.ConfirmDestructive(inputs, "drain node "+name); err != nil {
		return kubernetes.ErrorResult(err.Error()), nil
	}

	ignoreDaemonsets := boolWithDefault("ignore_daemonsets", inputs, true)
	deleteEmptyDir := boolWithDefault("delete_emptydir_data", inputs, false)
	gracePeriod, graceSet := kubernetes.OptionalInt("grace_period_seconds", inputs)
	timeoutSeconds, ok := kubernetes.OptionalInt("timeout_seconds", inputs)
	if !ok || timeoutSeconds <= 0 {
		timeoutSeconds = defaultTimeoutSeconds
	}

	// The drain can legitimately outlast a single API call (evicting many pods,
	// waiting out a PodDisruptionBudget), so it runs on its own budget rather than
	// kubernetes.Context(): the operator's timeout plus a minute of slack for the
	// in-flight request when the deadline lands mid-eviction.
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second+60*time.Second)
	defer cancel()

	// 1. Cordon first. Everything after this leaves the node cordoned on failure.
	cordon := []byte(`{"spec":{"unschedulable":true}}`)
	if _, err := kubernetes.Patch(ctx, auth, kubernetes.Nodes, "", name, cordon, kubernetes.PatchMerge); err != nil {
		return kubernetes.ErrorResult(fmt.Sprintf("failed to cordon node %s before draining: %s", name, err.Error())), nil
	}

	// 2. List every pod scheduled on the node, across all namespaces.
	pods, _, _, err := kubernetes.List(ctx, auth, kubernetes.Pods, "", kubernetes.ListOptions{
		FieldSelector: "spec.nodeName=" + name,
		ReturnAll:     true,
	})
	if err != nil {
		return kubernetes.ErrorResult(fmt.Sprintf("node %s was cordoned, but listing its pods failed: %s", name, err.Error())), nil
	}

	// 3. Classify. Skips are counted; blockers are collected so the operator sees
	//    every reason at once rather than fixing them one drain at a time.
	var toEvict []podRef
	skipped := 0
	var dsBlocked, emptyDirBlocked, unmanaged []string

	for _, it := range pods {
		pod, ok := it.(map[string]interface{})
		if !ok {
			continue
		}
		md := asMap(pod["metadata"])
		ns, _ := md["namespace"].(string)
		pn, _ := md["name"].(string)
		ref := ns + "/" + pn

		// Mirror pods are static; the kubelet owns them and the API server cannot
		// evict them, so kubectl simply skips them.
		if ann := asMap(md["annotations"]); ann != nil {
			if _, isMirror := ann["kubernetes.io/config.mirror"]; isMirror {
				skipped++
				continue
			}
		}

		if isDaemonSetPod(md) {
			if ignoreDaemonsets {
				skipped++
				continue
			}
			dsBlocked = append(dsBlocked, ref)
			continue
		}

		// Already terminating — no point evicting again.
		if md["deletionTimestamp"] != nil {
			skipped++
			continue
		}

		if !deleteEmptyDir && podHasEmptyDir(pod) {
			emptyDirBlocked = append(emptyDirBlocked, ref)
			continue
		}

		if !hasOwner(md) {
			unmanaged = append(unmanaged, ref)
			continue
		}

		toEvict = append(toEvict, podRef{namespace: ns, name: pn})
	}

	if blocked := blockMessage(name, dsBlocked, emptyDirBlocked, unmanaged); blocked != "" {
		out := kubernetes.ErrorResult(blocked)
		out["id"] = name
		out["evicted"] = 0
		out["skipped"] = skipped
		out["evicted_pods"] = []string{}
		return out, nil
	}

	// 4. Evict. deadline bounds the whole eviction phase; a PodDisruptionBudget
	//    that keeps returning 429 past it fails the drain (node stays cordoned).
	deadline := time.Now().Add(time.Duration(timeoutSeconds) * time.Second)
	evictedPods := []string{}
	for _, p := range toEvict {
		if err := evictPod(ctx, auth, p, gracePeriod, graceSet, deadline, timeoutSeconds); err != nil {
			out := kubernetes.ErrorResult(fmt.Sprintf(
				"drained %d of %d pod(s) from node %s before failing: %s. The node remains cordoned.",
				len(evictedPods), len(toEvict), name, err.Error()))
			out["id"] = name
			out["evicted"] = len(evictedPods)
			out["skipped"] = skipped
			out["evicted_pods"] = evictedPods
			return out, nil
		}
		evictedPods = append(evictedPods, p.namespace+"/"+p.name)
	}

	return map[string]interface{}{
		"id":           name,
		"evicted":      len(evictedPods),
		"skipped":      skipped,
		"evicted_pods": evictedPods,
		"tool_result":  fmt.Sprintf("Drained node %s: evicted %d pod(s), skipped %d", name, len(evictedPods), skipped),
		"success":      true,
		"error":        "",
	}, nil
}

// evictPod POSTs one Eviction and reconciles the outcome:
//
//   - 2xx: the API server accepted the eviction.
//   - 404: the pod is already gone — treat as success.
//   - 429: a PodDisruptionBudget is holding it back. Back off (1s, doubling, capped
//     at 10s) and retry until the deadline, then fail.
//   - anything else: a hard rejection, surfaced verbatim.
func evictPod(ctx context.Context, a kubernetes.Auth, p podRef, gracePeriod int, graceSet bool, deadline time.Time, timeoutSeconds int) error {
	body := map[string]interface{}{
		"apiVersion": "policy/v1",
		"kind":       "Eviction",
		"metadata": map[string]interface{}{
			"name":      p.name,
			"namespace": p.namespace,
		},
	}
	if graceSet {
		body["deleteOptions"] = map[string]interface{}{"gracePeriodSeconds": gracePeriod}
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	path, err := kubernetes.Pods.SubPath(p.namespace, p.name, "eviction")
	if err != nil {
		return err
	}

	backoff := time.Second
	for {
		resp, err := kubernetes.ExecuteAPI(ctx, a, http.MethodPost, path, raw, "application/json", "")
		if err != nil {
			return err
		}
		switch {
		case resp.StatusCode >= 200 && resp.StatusCode < 300:
			return nil
		case resp.StatusCode == http.StatusNotFound:
			return nil // already gone
		case resp.StatusCode == http.StatusTooManyRequests:
			if time.Now().After(deadline) {
				return fmt.Errorf("eviction of %s/%s is blocked by a PodDisruptionBudget and did not clear within the %ds timeout",
					p.namespace, p.name, timeoutSeconds)
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			if backoff *= 2; backoff > 10*time.Second {
				backoff = 10 * time.Second
			}
		default:
			return kubernetes.CheckResponse(resp)
		}
	}
}

// blockMessage assembles the single soft error that aborts a drain before any
// eviction, or "" when nothing blocks.
func blockMessage(node string, dsBlocked, emptyDirBlocked, unmanaged []string) string {
	var parts []string
	if len(dsBlocked) > 0 {
		parts = append(parts, fmt.Sprintf("%d DaemonSet-managed pod(s) (%s) — turn on “Ignore DaemonSets” to skip them",
			len(dsBlocked), strings.Join(dsBlocked, ", ")))
	}
	if len(emptyDirBlocked) > 0 {
		parts = append(parts, fmt.Sprintf("%d pod(s) with emptyDir volumes (%s) whose data would be lost — turn on “Delete emptyDir Data” to proceed",
			len(emptyDirBlocked), strings.Join(emptyDirBlocked, ", ")))
	}
	if len(unmanaged) > 0 {
		parts = append(parts, fmt.Sprintf("%d unmanaged pod(s) (%s) that nothing would recreate once evicted",
			len(unmanaged), strings.Join(unmanaged, ", ")))
	}
	if len(parts) == 0 {
		return ""
	}
	return fmt.Sprintf("refusing to drain node %s: %s. The node has been cordoned; no pods were evicted.",
		node, strings.Join(parts, "; "))
}

// isDaemonSetPod reports whether an ownerReferences entry names a DaemonSet.
func isDaemonSetPod(md map[string]interface{}) bool {
	for _, o := range asSlice(md["ownerReferences"]) {
		if om := asMap(o); om != nil {
			if kind, _ := om["kind"].(string); kind == "DaemonSet" {
				return true
			}
		}
	}
	return false
}

// hasOwner reports whether the pod has any ownerReferences at all — a controller
// (ReplicaSet, StatefulSet, Job…) that would recreate it elsewhere.
func hasOwner(md map[string]interface{}) bool {
	return len(asSlice(md["ownerReferences"])) > 0
}

// podHasEmptyDir reports whether any of the pod's volumes is an emptyDir, whose
// contents live only on this node and vanish when the pod is evicted.
func podHasEmptyDir(pod map[string]interface{}) bool {
	spec := asMap(pod["spec"])
	for _, v := range asSlice(spec["volumes"]) {
		if vm := asMap(v); vm != nil {
			if _, has := vm["emptyDir"]; has {
				return true
			}
		}
	}
	return false
}

// boolWithDefault reads a checkbox that has a documented default, returning the
// default when the input is absent. core.Connection carries no default, so the
// default lives in code; the parse still goes through kubernetes.BoolInput so a
// variable-bound value coerces correctly.
func boolWithDefault(name string, inputs []*core.Connection, def bool) bool {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Value == nil {
		return def
	}
	return kubernetes.BoolInput(name, inputs)
}

func asMap(v interface{}) map[string]interface{} {
	m, _ := v.(map[string]interface{})
	return m
}

func asSlice(v interface{}) []interface{} {
	s, _ := v.([]interface{})
	return s
}
