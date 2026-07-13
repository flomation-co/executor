// Package infrastructure_kubernetes_cronjob_trigger runs a CronJob once, now,
// off-schedule — the workflow equivalent of `kubectl create job --from=cronjob/X`.
//
// Kubernetes has no "trigger" or "instantiate" verb. What kubectl actually does,
// and what this action reproduces, is:
//
//  1. GET the CronJob.
//  2. Take its spec.jobTemplate: the template's .spec becomes the new Job's
//     .spec verbatim, and its .metadata annotations/labels carry over.
//  3. POST a fresh Job carrying:
//     - a name (supplied, or "<cronjob>-manual-<unix seconds>"), capped at the
//     63-character label limit;
//     - the annotation cronjob.kubernetes.io/instantiate=manual, which is how
//     Kubernetes marks a hand-run Job and how the CronJob controller knows not
//     to count it against the schedule's missed-run accounting;
//     - an ownerReference back to the CronJob.
//
// The ownerReference is the load-bearing detail. Without it the Job is an orphan
// that outlives `kubectl delete cronjob`; with it, garbage collection ties the
// Job's lifetime to the CronJob's, exactly as kubectl's version does.
package infrastructure_kubernetes_cronjob_trigger

import (
	"fmt"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/kubernetes"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "Kubernetes Trigger CronJob"
	Description  = "Run a CronJob immediately, off-schedule, by creating a one-off Job from its template — the same as kubectl create job --from=cronjob."
	Website      = "https://www.flomation.co"
	Icon         = "kubernetes+play"
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

	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace", Placeholder: "The namespace the cronjob lives in", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "CronJob", Placeholder: "The cronjob to run now", Required: true},
	{Name: "job_name", Type: core.ConnectionTypeString, Label: "Job Name", Placeholder: "Name for the one-off Job (default <cronjob>-manual-<timestamp>)"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Job Name"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Job"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// maxNameLength is the DNS label limit a Job's metadata.name must obey.
const maxNameLength = 63

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

	ctx, cancel := kubernetes.Context()
	defer cancel()

	cronJob, err := kubernetes.Get(ctx, auth, kubernetes.CronJobs, namespace, name)
	if err != nil {
		return kubernetes.ErrorResult(err.Error()), nil
	}

	// Reading through nil maps is safe in Go, so these assertions never panic —
	// a missing layer just leaves jobSpec empty, which we reject below.
	cronSpec, _ := cronJob["spec"].(map[string]interface{})
	jobTemplate, _ := cronSpec["jobTemplate"].(map[string]interface{})
	jobSpec, _ := jobTemplate["spec"].(map[string]interface{})
	if len(jobSpec) == 0 {
		return kubernetes.ErrorResult(fmt.Sprintf(
			"CronJob %s has no spec.jobTemplate.spec to run — it may be a legacy batch/v1beta1 object this node cannot read", name)), nil
	}

	// Carry over the template's annotations/labels, then stamp the manual-run
	// marker on top, matching kubectl.
	templateMeta, _ := jobTemplate["metadata"].(map[string]interface{})

	annotations := map[string]interface{}{}
	if existing, ok := templateMeta["annotations"].(map[string]interface{}); ok {
		for k, v := range existing {
			annotations[k] = v
		}
	}
	annotations[kubernetes.InstantiateAnnotation] = "manual"

	labels := map[string]interface{}{}
	if existing, ok := templateMeta["labels"].(map[string]interface{}); ok {
		for k, v := range existing {
			labels[k] = v
		}
	}

	cronMeta, _ := cronJob["metadata"].(map[string]interface{})
	uid, _ := cronMeta["uid"].(string)

	jobName := kubernetes.OptionalString("job_name", inputs)
	if jobName == "" {
		jobName = fmt.Sprintf("%s-manual-%d", name, time.Now().Unix())
	}
	if len(jobName) > maxNameLength {
		jobName = jobName[:maxNameLength]
	}
	// A truncation (or a pasted name) that ends on a hyphen/dot is not a valid
	// DNS label; trimming avoids a confusing 422 from the API server.
	jobName = strings.TrimRight(jobName, "-.")

	jobMeta := map[string]interface{}{
		"name":        jobName,
		"annotations": annotations,
		"ownerReferences": []interface{}{
			map[string]interface{}{
				"apiVersion": "batch/v1",
				"kind":       "CronJob",
				"name":       name,
				"uid":        uid,
			},
		},
	}
	if len(labels) > 0 {
		jobMeta["labels"] = labels
	}

	job := map[string]interface{}{
		"apiVersion": "batch/v1",
		"kind":       "Job",
		"metadata":   jobMeta,
		"spec":       jobSpec,
	}

	created, err := kubernetes.Create(ctx, auth, kubernetes.Jobs, namespace, job)
	if err != nil {
		return kubernetes.ErrorResult(err.Error()), nil
	}

	out := kubernetes.ObjectResult(created, fmt.Sprintf("Triggered CronJob %s — created Job %s", name, jobName))
	if kubernetes.ObjectName(created) == "" {
		out["id"] = jobName
	}
	return out, nil
}
