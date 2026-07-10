// Package infrastructure_kubernetes_job_create creates a batch/v1 Job that runs a
// single container to completion.
//
// A Job is the right primitive for one-off and scheduled-by-the-flow work — a
// migration, a backup, a report — where Kubernetes should retry on failure
// (backoffLimit) and, optionally, garbage-collect the finished Job after a
// window (ttlSecondsAfterFinished) so the namespace does not fill with
// tombstones. restartPolicy is constrained to Never or OnFailure because a Job's
// pod template rejects Always: a Job that restarted forever would never complete.
//
// The Command field is a shell-style string, tokenised into an argv by
// kubernetes.SplitCommand — the same splitter cronjob_create uses, so a Job
// created here and a Job spawned from a CronJob template run identical argv for
// identical input. It is NOT run through a shell: no globbing, no variable
// expansion, no pipes. To use those, set the command to `sh -c "…"` explicitly.
// An unbalanced quote is a hard config error, caught here rather than surfacing
// as an opaque container start failure.
package infrastructure_kubernetes_job_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/kubernetes"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "Kubernetes Create Job"
	Description  = "Create a Job that runs a container image to completion, with retry, TTL cleanup, and an optional command and environment."
	Website      = "https://www.flomation.co"
	Icon         = "kubernetes+plus"
	Date         = "10/07/2026"
	Type         = core.ActionTypeAction
)

// defaultBackoffLimit matches the Kubernetes default: the Job retries a failing
// pod up to this many times before it is marked Failed.
const defaultBackoffLimit = 3

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

	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace", Placeholder: "The namespace to create the job in", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Job Name", Placeholder: "A DNS-safe name, e.g. nightly-backup", Required: true},
	{Name: "image", Type: core.ConnectionTypeString, Label: "Container Image", Placeholder: "e.g. busybox:1.36 or registry.example.com/backup:latest", Required: true},
	{Name: "command", Type: core.ConnectionTypeString, Label: "Command", Placeholder: "Shell-style command; leave blank to use the image entrypoint"},
	{Name: "env", Type: core.ConnectionTypeObject, Label: "Environment Variables", Placeholder: `{"LOG_LEVEL":"debug","REGION":"eu-west-1"}`},
	{Name: "backoff_limit", Type: core.ConnectionTypeInteger, Label: "Backoff Limit", Placeholder: "Retries before the job is marked failed (default 3)"},
	{Name: "ttl_seconds_after_finished", Type: core.ConnectionTypeInteger, Label: "TTL After Finished (seconds)", Placeholder: "Auto-delete the finished job after N seconds (blank to keep it)"},
	{Name: "restart_policy", Type: core.ConnectionTypeString, Label: "Restart Policy", Options: []core.ConnectionOption{
		{Name: "Never", Value: "Never"},
		{Name: "On Failure", Value: "OnFailure"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Job Name"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Job"},
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
	image, err := kubernetes.RequiredString("image", inputs)
	if err != nil {
		return nil, err
	}

	restartPolicy := kubernetes.OptionalString("restart_policy", inputs)
	if restartPolicy == "" {
		restartPolicy = "Never"
	}
	if restartPolicy != "Never" && restartPolicy != "OnFailure" {
		return nil, fmt.Errorf(`restart_policy must be "Never" or "OnFailure" (got %q)`, restartPolicy)
	}

	var command []string
	if raw := kubernetes.OptionalString("command", inputs); raw != "" {
		command, err = kubernetes.SplitCommand(raw)
		if err != nil {
			return nil, err
		}
	}

	envMap, err := kubernetes.StringMapInput("env", inputs)
	if err != nil {
		return nil, err
	}
	env := kubernetes.EnvList(envMap)

	backoffLimit := defaultBackoffLimit
	if v, ok := kubernetes.OptionalInt("backoff_limit", inputs); ok {
		backoffLimit = v
	}

	container := map[string]interface{}{
		"name":  name,
		"image": image,
	}
	if len(command) > 0 {
		container["command"] = command
	}
	if len(env) > 0 {
		container["env"] = env
	}

	spec := map[string]interface{}{
		"backoffLimit": backoffLimit,
		"template": map[string]interface{}{
			"spec": map[string]interface{}{
				"restartPolicy": restartPolicy,
				"containers":    []map[string]interface{}{container},
			},
		},
	}
	if ttl, ok := kubernetes.OptionalInt("ttl_seconds_after_finished", inputs); ok {
		spec["ttlSecondsAfterFinished"] = ttl
	}

	body := map[string]interface{}{
		"apiVersion": kubernetes.Jobs.APIVersion(),
		"kind":       kubernetes.Jobs.Kind,
		"metadata":   map[string]interface{}{"name": name},
		"spec":       spec,
	}

	ctx, cancel := kubernetes.Context()
	defer cancel()

	obj, err := kubernetes.Create(ctx, auth, kubernetes.Jobs, namespace, body)
	if err != nil {
		return kubernetes.ErrorResult(err.Error()), nil
	}

	return kubernetes.ObjectResult(obj, fmt.Sprintf("Created job %s in namespace %s", name, namespace)), nil
}
