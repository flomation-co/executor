// Resource registry: the (group, version, plural, namespaced) tuple for every
// Kubernetes kind the actions touch, plus the URL algebra that turns one into a
// REST path.
//
// Kubernetes splits its REST surface in two and the shapes are NOT the same:
//
//	core group ("")   →  /api/v1/...
//	named group       →  /apis/<group>/<version>/...
//
// and orthogonally, namespaced kinds insert /namespaces/<ns> before the plural
// while cluster-scoped kinds (Node, Namespace) do not. Getting either wrong
// yields a 404 that reads like "the object doesn't exist", so the mapping lives
// here once rather than being spelled out in eighty action files.
package kubernetes

import (
	"fmt"
	"net/url"
	"strings"
)

// Resource identifies a Kubernetes kind well enough to address it over REST.
type Resource struct {
	// Group is the API group, "" for the core group (pods, services, nodes…).
	Group string
	// Version is the group version served by the cluster (v1, v2…).
	Version string
	// Plural is the lowercase resource name as it appears in the URL path.
	Plural string
	// Kind is the CamelCase kind, used when building object bodies.
	Kind string
	// Namespaced is false for cluster-scoped kinds (Node, Namespace).
	Namespaced bool
}

// The kinds the Kubernetes actions address. autoscaling/v2 for HPA is the GA
// version from Kubernetes 1.23 onward (v2beta2 was removed in 1.26).
var (
	Namespaces      = Resource{Group: "", Version: "v1", Plural: "namespaces", Kind: "Namespace", Namespaced: false}
	Nodes           = Resource{Group: "", Version: "v1", Plural: "nodes", Kind: "Node", Namespaced: false}
	Pods            = Resource{Group: "", Version: "v1", Plural: "pods", Kind: "Pod", Namespaced: true}
	Services        = Resource{Group: "", Version: "v1", Plural: "services", Kind: "Service", Namespaced: true}
	ConfigMaps      = Resource{Group: "", Version: "v1", Plural: "configmaps", Kind: "ConfigMap", Namespaced: true}
	Secrets         = Resource{Group: "", Version: "v1", Plural: "secrets", Kind: "Secret", Namespaced: true}
	Events          = Resource{Group: "", Version: "v1", Plural: "events", Kind: "Event", Namespaced: true}
	PVCs            = Resource{Group: "", Version: "v1", Plural: "persistentvolumeclaims", Kind: "PersistentVolumeClaim", Namespaced: true}
	ServiceAccounts = Resource{Group: "", Version: "v1", Plural: "serviceaccounts", Kind: "ServiceAccount", Namespaced: true}

	Deployments  = Resource{Group: "apps", Version: "v1", Plural: "deployments", Kind: "Deployment", Namespaced: true}
	StatefulSets = Resource{Group: "apps", Version: "v1", Plural: "statefulsets", Kind: "StatefulSet", Namespaced: true}
	DaemonSets   = Resource{Group: "apps", Version: "v1", Plural: "daemonsets", Kind: "DaemonSet", Namespaced: true}
	ReplicaSets  = Resource{Group: "apps", Version: "v1", Plural: "replicasets", Kind: "ReplicaSet", Namespaced: true}

	Jobs     = Resource{Group: "batch", Version: "v1", Plural: "jobs", Kind: "Job", Namespaced: true}
	CronJobs = Resource{Group: "batch", Version: "v1", Plural: "cronjobs", Kind: "CronJob", Namespaced: true}

	Ingresses = Resource{Group: "networking.k8s.io", Version: "v1", Plural: "ingresses", Kind: "Ingress", Namespaced: true}
	HPAs      = Resource{Group: "autoscaling", Version: "v2", Plural: "horizontalpodautoscalers", Kind: "HorizontalPodAutoscaler", Namespaced: true}
)

// APIVersion renders the value that belongs in an object's apiVersion field:
// bare "v1" for the core group, "group/version" otherwise.
func (r Resource) APIVersion() string {
	if r.Group == "" {
		return r.Version
	}
	return r.Group + "/" + r.Version
}

// APIRoot is the path prefix for the resource's group: /api/v1 for the core
// group, /apis/<group>/<version> for every named group.
func (r Resource) APIRoot() string {
	if r.Group == "" {
		return "/api/" + r.Version
	}
	return "/apis/" + r.Group + "/" + r.Version
}

// Path builds the REST path for a collection (name == "") or a single object.
//
// A namespaced kind requires a namespace; a cluster-scoped kind rejects one
// rather than silently ignoring it, because a caller that passes a namespace to
// Nodes has misunderstood something and would otherwise get a confusing 404 for
// a node that plainly exists.
//
// Both segments are escaped. Kubernetes names are restricted to a DNS-safe
// alphabet, so escaping is belt-and-braces against an operator pasting a name
// with a slash and traversing to another collection.
func (r Resource) Path(namespace, name string) (string, error) {
	var b strings.Builder
	b.WriteString(r.APIRoot())

	if r.Namespaced {
		ns := strings.TrimSpace(namespace)
		if ns == "" {
			return "", fmt.Errorf("namespace is required for %s", r.Plural)
		}
		b.WriteString("/namespaces/")
		b.WriteString(url.PathEscape(ns))
	} else if strings.TrimSpace(namespace) != "" {
		return "", fmt.Errorf("%s is cluster-scoped and takes no namespace", r.Plural)
	}

	b.WriteString("/")
	b.WriteString(r.Plural)

	if n := strings.TrimSpace(name); n != "" {
		b.WriteString("/")
		b.WriteString(url.PathEscape(n))
	}
	return b.String(), nil
}

// SubPath builds the path to a subresource of a single object, e.g. the /scale
// of a Deployment, the /log of a Pod, or the /eviction of a Pod.
func (r Resource) SubPath(namespace, name, sub string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("name is required to address the %s subresource of a %s", sub, r.Kind)
	}
	base, err := r.Path(namespace, name)
	if err != nil {
		return "", err
	}
	return base + "/" + sub, nil
}

// Patch content types. Kubernetes dispatches on Content-Type, and the wrong one
// is a 415 or — worse — a silently different merge semantic:
//
//   - StrategicMerge understands a type's patchStrategy tags, so it merges list
//     items by key instead of replacing the whole list. Only built-in types
//     support it. This is what `kubectl rollout restart` uses.
//   - Merge (RFC 7386) replaces lists wholesale. It is the only merge patch the
//     /scale subresource accepts — Scale carries no patchStrategy metadata, so
//     a strategic-merge patch against it is rejected.
//   - JSONPatch (RFC 6902) is an explicit op list, used where a change must be
//     conditional on a path already existing.
//   - Apply is Server-Side Apply; the body is a partial object and the server
//     tracks field ownership under the supplied fieldManager.
const (
	PatchStrategicMerge = "application/strategic-merge-patch+json"
	PatchMerge          = "application/merge-patch+json"
	PatchJSON           = "application/json-patch+json"
	PatchApply          = "application/apply-patch+yaml"
)

// RestartedAtAnnotation is the annotation `kubectl rollout restart` stamps onto
// a pod template to force a rollout. Nothing reads its value — mutating the pod
// template at all is what bumps the controller's revision and triggers a rolling
// replacement. Reusing kubectl's exact key means a Flomation-triggered restart is
// indistinguishable from a kubectl one in the resulting revision history.
const RestartedAtAnnotation = "kubectl.kubernetes.io/restartedAt"

// InstantiateAnnotation marks a Job created by hand from a CronJob's template,
// matching `kubectl create job --from=cronjob/<name>`.
const InstantiateAnnotation = "cronjob.kubernetes.io/instantiate"

// FieldManager identifies Flomation as the owner of fields it applies, so a
// later Server-Side Apply from this node can take ownership back without the
// conflict a differing manager name would raise.
const FieldManager = "flomation-automate"
