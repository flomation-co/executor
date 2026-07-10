// Reading Helm release state without Helm.
//
// The Helm 3 client stores each release revision as a Kubernetes Secret in the
// release's own namespace, under the default "secret" storage driver:
//
//	name    sh.helm.release.v1.<release>.v<revision>
//	type    helm.sh/release.v1
//	labels  owner=helm, name=<release>, version=<revision>, status=<status>
//	data    release: base64( base64( gzip( json ) ) )
//
// The double base64 is not a typo. Helm encodes the gzipped payload to base64
// itself, then Kubernetes base64-encodes every Secret value on the wire — so a
// caller reading .data.release decodes twice before it reaches gzip's 1f 8b.
// (Very old releases were stored ungzipped; the magic-byte check below keeps
// those readable rather than failing on a bad gzip header.)
//
// Because the labels carry the release name, revision and status, listing
// releases needs no payload decoding at all — only the winning revision of each
// release is decoded, for its chart version and timestamps. That keeps
// release_list cheap on a cluster with hundreds of revisions.
//
// This mirrors what `helm list` does, and deliberately supports only the default
// secret driver. A cluster configured with HELM_DRIVER=configmap or sql stores
// its state elsewhere and will simply report no releases.
package helm

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"flomation.app/automate/executor/actions/infrastructure/kubernetes"
)

// HelmStorageLabel selects every Secret written by the Helm storage driver.
const HelmStorageLabel = "owner=helm"

// maxReleasePayload bounds a decompressed release document. A release carries its
// full rendered manifest and every value, so a big chart is legitimately a few
// megabytes; beyond this a gzip bomb is the likelier explanation.
const maxReleasePayload = 32 << 20 // 32 MB

// Release is a decoded Helm release revision.
type Release struct {
	Name      string                 `json:"name"`
	Namespace string                 `json:"namespace"`
	Version   int                    `json:"version"`
	Info      map[string]interface{} `json:"info"`
	Chart     map[string]interface{} `json:"chart"`
	Config    map[string]interface{} `json:"config"`
	Manifest  string                 `json:"manifest"`
	Hooks     []interface{}          `json:"hooks"`
}

// Status is the release's lifecycle state: deployed, superseded, failed,
// uninstalling, pending-install, pending-upgrade, pending-rollback.
func (r *Release) Status() string {
	if r == nil || r.Info == nil {
		return ""
	}
	s, _ := r.Info["status"].(string)
	return s
}

// Notes is the rendered NOTES.txt shown after an install.
func (r *Release) Notes() string {
	if r == nil || r.Info == nil {
		return ""
	}
	n, _ := r.Info["notes"].(string)
	return n
}

// ChartName renders the chart's "name-version" as `helm list` displays it.
func (r *Release) ChartName() string {
	md := r.chartMetadata()
	if md == nil {
		return ""
	}
	name, _ := md["name"].(string)
	version, _ := md["version"].(string)
	if name == "" {
		return ""
	}
	if version == "" {
		return name
	}
	return name + "-" + version
}

// AppVersion is the application version the chart deploys.
func (r *Release) AppVersion() string {
	md := r.chartMetadata()
	if md == nil {
		return ""
	}
	v, _ := md["appVersion"].(string)
	return v
}

func (r *Release) chartMetadata() map[string]interface{} {
	if r == nil || r.Chart == nil {
		return nil
	}
	md, _ := r.Chart["metadata"].(map[string]interface{})
	return md
}

// Summary is the flattened row shape release_list emits — the columns `helm list`
// prints, without the full manifest and values payload weighing down the output.
func (r *Release) Summary() map[string]interface{} {
	info := func(k string) string {
		if r.Info == nil {
			return ""
		}
		v, _ := r.Info[k].(string)
		return v
	}
	return map[string]interface{}{
		"name":        r.Name,
		"namespace":   r.Namespace,
		"revision":    r.Version,
		"status":      r.Status(),
		"chart":       r.ChartName(),
		"app_version": r.AppVersion(),
		"updated":     info("last_deployed"),
		"description": info("description"),
	}
}

// DecodeReleaseSecret extracts a Release from a decoded Kubernetes Secret object.
//
// The Secret arrives as generic JSON from the REST client, so .data.release is
// already a Go string holding the *outer* base64 — Kubernetes' own encoding of
// the value. Both layers are peeled here.
func DecodeReleaseSecret(secret map[string]interface{}) (*Release, error) {
	data, ok := secret["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("secret %q has no data", kubernetes.ObjectName(secret))
	}
	outer, ok := data["release"].(string)
	if !ok || outer == "" {
		return nil, fmt.Errorf("secret %q has no release payload", kubernetes.ObjectName(secret))
	}

	// Layer 1: Kubernetes' base64 of the Secret value.
	inner, err := base64.StdEncoding.DecodeString(outer)
	if err != nil {
		return nil, fmt.Errorf("release payload is not valid base64: %w", err)
	}
	// Layer 2: Helm's own base64 of the gzipped document.
	raw, err := base64.StdEncoding.DecodeString(string(inner))
	if err != nil {
		// Helm <3.0.0-beta stored the document without the second encoding.
		raw = inner
	}

	// Helm gzips by default but tolerates an uncompressed document; branch on the
	// magic bytes rather than on a version we cannot see.
	if len(raw) > 3 && raw[0] == 0x1f && raw[1] == 0x8b {
		zr, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return nil, fmt.Errorf("release payload is not readable gzip: %w", err)
		}
		defer func() { _ = zr.Close() }()

		decompressed, err := io.ReadAll(io.LimitReader(zr, maxReleasePayload+1))
		if err != nil {
			return nil, fmt.Errorf("could not decompress release payload: %w", err)
		}
		if len(decompressed) > maxReleasePayload {
			return nil, fmt.Errorf("release payload exceeds %d bytes", maxReleasePayload)
		}
		raw = decompressed
	}

	var rel Release
	if err := json.Unmarshal(raw, &rel); err != nil {
		return nil, fmt.Errorf("could not parse release payload: %w", err)
	}
	return &rel, nil
}

// secretRevision reads the revision from a release Secret's labels, which is far
// cheaper than decoding the payload to find release.version.
func secretRevision(secret map[string]interface{}) int {
	md, ok := secret["metadata"].(map[string]interface{})
	if !ok {
		return 0
	}
	labels, ok := md["labels"].(map[string]interface{})
	if !ok {
		return 0
	}
	v, _ := labels["version"].(string)
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return n
}

func secretLabel(secret map[string]interface{}, key string) string {
	md, ok := secret["metadata"].(map[string]interface{})
	if !ok {
		return ""
	}
	labels, ok := md["labels"].(map[string]interface{})
	if !ok {
		return ""
	}
	v, _ := labels[key].(string)
	return v
}

// listReleaseSecrets fetches every Helm storage Secret in a namespace (or across
// all namespaces when namespace is ""), optionally narrowed to one release.
func listReleaseSecrets(ctx context.Context, a kubernetes.Auth, namespace, release string) ([]interface{}, error) {
	selector := HelmStorageLabel
	if r := strings.TrimSpace(release); r != "" {
		selector += ",name=" + r
	}
	items, _, _, err := kubernetes.List(ctx, a, kubernetes.Secrets, namespace, kubernetes.ListOptions{
		LabelSelector: selector,
		ReturnAll:     true,
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}

// LatestRevisions returns the newest revision of every release in a namespace,
// keyed by "<namespace>/<name>". This is what `helm list` shows: one row per
// release, at its current revision, whatever that revision's status.
//
// Only the winning Secret of each release is decoded — the rest are discarded on
// their labels alone.
func LatestRevisions(ctx context.Context, a kubernetes.Auth, namespace string) ([]*Release, error) {
	secrets, err := listReleaseSecrets(ctx, a, namespace, "")
	if err != nil {
		return nil, err
	}

	winners := map[string]map[string]interface{}{}
	for _, it := range secrets {
		secret, ok := it.(map[string]interface{})
		if !ok {
			continue
		}
		name := secretLabel(secret, "name")
		if name == "" {
			continue
		}
		key := kubernetes.ObjectNamespace(secret) + "/" + name
		if cur, seen := winners[key]; !seen || secretRevision(secret) > secretRevision(cur) {
			winners[key] = secret
		}
	}

	out := make([]*Release, 0, len(winners))
	for _, secret := range winners {
		rel, err := DecodeReleaseSecret(secret)
		if err != nil {
			return nil, err
		}
		out = append(out, rel)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// History returns every stored revision of one release, oldest first.
func History(ctx context.Context, a kubernetes.Auth, namespace, release string) ([]*Release, error) {
	if strings.TrimSpace(release) == "" {
		return nil, fmt.Errorf("release name is required")
	}
	secrets, err := listReleaseSecrets(ctx, a, namespace, release)
	if err != nil {
		return nil, err
	}
	if len(secrets) == 0 {
		return nil, fmt.Errorf("release %q not found in namespace %q", release, namespace)
	}

	out := make([]*Release, 0, len(secrets))
	for _, it := range secrets {
		secret, ok := it.(map[string]interface{})
		if !ok {
			continue
		}
		rel, err := DecodeReleaseSecret(secret)
		if err != nil {
			return nil, err
		}
		out = append(out, rel)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

// Revision returns one revision of a release. revision <= 0 selects the newest.
func Revision(ctx context.Context, a kubernetes.Auth, namespace, release string, revision int) (*Release, error) {
	all, err := History(ctx, a, namespace, release)
	if err != nil {
		return nil, err
	}
	if revision <= 0 {
		return all[len(all)-1], nil
	}
	for _, rel := range all {
		if rel.Version == revision {
			return rel, nil
		}
	}
	return nil, fmt.Errorf("release %q has no revision %d in namespace %q", release, revision, namespace)
}
