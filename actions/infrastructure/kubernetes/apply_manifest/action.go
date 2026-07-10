// Package infrastructure_kubernetes_apply_manifest is the node's advanced escape
// hatch: it applies ANY Kubernetes object — including kinds this node ships no
// typed action for, such as CRDs, an Ingress with rules, RBAC Roles and
// RoleBindings, or a PodDisruptionBudget — from a single YAML or JSON document.
//
// It uses Server-Side Apply: a PATCH with Content-Type application/apply-patch+yaml
// (kubernetes.PatchApply). SSA is declarative and idempotent — the same manifest
// applied twice converges to the same object, and a PATCH against a name that does
// not exist yet CREATES it, so there is no separate create path. Applying makes
// Flomation (fieldManager=flomation-automate) the recorded owner of exactly the
// fields the manifest sets; fields it omits are left untouched, and re-applying
// with a field removed relinquishes ownership of that field.
//
// force_conflicts steals ownership. When another manager — typically an operator
// or controller that reconciles the same object — already owns a field this
// manifest also sets, SSA refuses with a conflict unless force is set. Forcing
// makes Flomation the owner, and the other controller's next reconcile may well
// fight back. Leave it off unless you mean to take a field away from whatever
// currently manages it.
//
// Discovery, not a hardcoded table, resolves a kind's plural and whether it is
// namespaced (GET /api/v1 for the core group, else /apis/<group>/<version>) — that
// is what lets this one action address a CRD the node has never heard of. The
// discovery list reports subresources (deployments/status) under the parent's
// kind, so entries whose name contains a "/" are skipped when matching.
//
// The manifest bytes go to the API server verbatim: re-serialising through a Go
// map would reorder fields and reformat numbers, and the server — not this code —
// is the thing that parses them. JSON is valid YAML, so a JSON manifest passes
// through unchanged.
package infrastructure_kubernetes_apply_manifest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	yaml "go.yaml.in/yaml/v3"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/kubernetes"
)

const (
	Author       = "David McElin"
	Organisation = "Flomation"
	Name         = "Kubernetes Apply Manifest"
	Description  = "Advanced: apply any Kubernetes object from a YAML or JSON manifest via Server-Side Apply — including CRDs and kinds with no dedicated action. Creates or updates in place."
	Website      = "https://www.flomation.co"
	Icon         = "kubernetes+code"
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

	{Name: "manifest", Type: core.ConnectionTypeCode, Label: "Manifest", Placeholder: "A single Kubernetes object as YAML or JSON — must set apiVersion, kind and metadata.name", Required: true},
	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace", Placeholder: "Overrides the manifest's metadata.namespace; ignored for cluster-scoped kinds"},
	{Name: "dry_run", Type: core.ConnectionTypeBoolean, Label: "Dry Run", Placeholder: "Validate against the API server without persisting any change"},
	{Name: "force_conflicts", Type: core.ConnectionTypeBoolean, Label: "Force Conflicts", Placeholder: "Take ownership of fields another controller manages"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Object Name"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Object"},
	{Name: "kind", Type: core.ConnectionTypeString, Label: "Kind"},
	{Name: "applied", Type: core.ConnectionTypeBoolean, Label: "Applied"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// routingDoc is the minimal slice of a manifest needed to address it: enough to
// pick the discovery group/version and to build the object's URL. Everything else
// in the document is the API server's problem, and is never decoded here.
type routingDoc struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"metadata"`
}

// apiResourceList is the shape GET /api/v1 (or /apis/<group>/<version>) returns.
type apiResourceList struct {
	Resources []struct {
		Name       string `json:"name"`
		Kind       string `json:"kind"`
		Namespaced bool   `json:"namespaced"`
	} `json:"resources"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := kubernetes.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	manifest, err := readManifest(inputs)
	if err != nil {
		return nil, err
	}

	// A "---" at the start of a line separates YAML documents. This action addresses
	// exactly one object (one name, one URL), so a bundle has no single answer.
	if strings.Contains(strings.TrimSpace(manifest), "\n---") {
		return nil, fmt.Errorf("apply one object at a time: this manifest has a “---” document separator. Split it and give each object its own Apply Manifest step")
	}

	var doc routingDoc
	if err := yaml.Unmarshal([]byte(manifest), &doc); err != nil {
		return nil, fmt.Errorf("manifest is not valid YAML or JSON: %w", err)
	}
	doc.APIVersion = strings.TrimSpace(doc.APIVersion)
	doc.Kind = strings.TrimSpace(doc.Kind)
	doc.Metadata.Name = strings.TrimSpace(doc.Metadata.Name)
	switch {
	case doc.APIVersion == "":
		return nil, fmt.Errorf("manifest is missing apiVersion (e.g. apps/v1, networking.k8s.io/v1, or v1 for the core group)")
	case doc.Kind == "":
		return nil, fmt.Errorf("manifest is missing kind (e.g. Deployment, Ingress, or your CRD's kind)")
	case doc.Metadata.Name == "":
		return nil, fmt.Errorf("manifest is missing metadata.name")
	}

	// apiVersion is "group/version" for a named group, or a bare "version" for the
	// core group.
	group, version := "", doc.APIVersion
	if i := strings.Index(doc.APIVersion, "/"); i >= 0 {
		group, version = doc.APIVersion[:i], doc.APIVersion[i+1:]
	}

	ctx, cancel := kubernetes.Context()
	defer cancel()

	// Discover the kind's plural and scope. Reusing Resource.APIRoot() gives the
	// right discovery endpoint for both the core group and a named group.
	discovery := kubernetes.Resource{Group: group, Version: version}
	resp, err := kubernetes.ExecuteAPI(ctx, auth, http.MethodGet, discovery.APIRoot(), nil, "", "")
	if err != nil {
		return kubernetes.ErrorResult(err.Error()), nil
	}
	if err := kubernetes.CheckResponse(resp); err != nil {
		return kubernetes.ErrorResult(fmt.Sprintf("could not look up %q on the cluster: %s", doc.APIVersion, err.Error())), nil
	}
	var list apiResourceList
	if err := json.Unmarshal(resp.Body, &list); err != nil {
		return kubernetes.ErrorResult(fmt.Sprintf("could not parse the discovery response for %q: %s", doc.APIVersion, err.Error())), nil
	}

	var res *kubernetes.Resource
	for _, entry := range list.Resources {
		// A "/" in the name marks a subresource (deployments/status), which reports
		// the parent's kind; matching it would build the wrong URL.
		if strings.Contains(entry.Name, "/") {
			continue
		}
		if entry.Kind == doc.Kind {
			res = &kubernetes.Resource{Group: group, Version: version, Plural: entry.Name, Kind: entry.Kind, Namespaced: entry.Namespaced}
			break
		}
	}
	if res == nil {
		return kubernetes.ErrorResult(fmt.Sprintf("the cluster serves no %q resource in %s — check the kind and apiVersion, and that the CRD is installed", doc.Kind, doc.APIVersion)), nil
	}

	// Resolve the namespace for the URL. The input overrides the manifest; when the
	// kind is cluster-scoped both are ignored.
	namespace := ""
	if res.Namespaced {
		namespace = kubernetes.OptionalString("namespace", inputs)
		if namespace == "" {
			namespace = strings.TrimSpace(doc.Metadata.Namespace)
		}
		if namespace == "" {
			return nil, fmt.Errorf("%s is namespaced — set the Namespace input or metadata.namespace in the manifest", doc.Kind)
		}
	}

	path, err := res.Path(namespace, doc.Metadata.Name)
	if err != nil {
		return kubernetes.ErrorResult(err.Error()), nil
	}

	dryRun := kubernetes.BoolInput("dry_run", inputs)
	path += "?fieldManager=" + kubernetes.FieldManager
	if kubernetes.BoolInput("force_conflicts", inputs) {
		path += "&force=true"
	}
	if dryRun {
		path += "&dryRun=All"
	}

	resp, err = kubernetes.ExecuteAPI(ctx, auth, http.MethodPatch, path, []byte(manifest), kubernetes.PatchApply, "")
	if err != nil {
		return kubernetes.ErrorResult(err.Error()), nil
	}
	if err := kubernetes.CheckResponse(resp); err != nil {
		return kubernetes.ErrorResult(err.Error()), nil
	}
	obj, err := kubernetes.Decode(resp)
	if err != nil {
		return kubernetes.ErrorResult(err.Error()), nil
	}

	name := kubernetes.ObjectName(obj)
	if name == "" {
		name = doc.Metadata.Name
	}

	scope := doc.Kind + " " + name
	if res.Namespaced {
		scope += " in namespace " + namespace
	}
	summary := "Applied " + scope
	if dryRun {
		summary = "Dry run: " + scope + " is valid — nothing was changed"
	}

	return map[string]interface{}{
		"id":          name,
		"result":      obj,
		"kind":        doc.Kind,
		"applied":     !dryRun,
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}, nil
}

// readManifest pulls the manifest document out of the Code input.
//
// It reads the raw value rather than kubernetes.OptionalString because the body
// must reach the API server verbatim — the server, not this code, parses it, and a
// round-trip through a Go map would drop field ordering and reformat numbers. When
// an upstream node's whole object output is wired straight into this input it
// arrives already decoded (a map, not text); JSON is valid YAML, so re-encoding it
// yields a document the server accepts unchanged.
func readManifest(inputs []*core.Connection) (string, error) {
	conn := core.FindConnection("manifest", inputs)
	if conn == nil || conn.Value == nil {
		return "", fmt.Errorf("manifest is required — paste a single Kubernetes object as YAML or JSON")
	}
	var manifest string
	if s, ok := conn.Value.(string); ok {
		manifest = s
	} else {
		b, err := json.Marshal(conn.Value)
		if err != nil {
			return "", fmt.Errorf("manifest could not be read as a document: %w", err)
		}
		manifest = string(b)
	}
	if strings.TrimSpace(manifest) == "" {
		return "", fmt.Errorf("manifest is required — paste a single Kubernetes object as YAML or JSON")
	}
	return manifest, nil
}
