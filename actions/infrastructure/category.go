// Package infrastructure declares the top-level "Infrastructure" action
// category — the home for provisioning and operating the platform a workload
// runs on.
//
// It already had one member before this package existed: the OpenTofu actions
// live at actions/opentofu/* and are folded into this same category by the
// api's categoryMetadata map, which remaps the "opentofu" key onto
// Key: "infrastructure". Those are two-segment action IDs (opentofu/apply);
// the members declared under THIS directory use three-segment IDs
// (infrastructure/kubernetes/pod_list, infrastructure/helm/release_install)
// and resolve their sub-group from the api's subCategoryMetadata map.
//
// Because both shapes emit the same Key ("infrastructure") they feed the same
// group header in the editor palette. The api's top-level "infrastructure"
// entry and the inline "opentofu" remap must therefore carry byte-identical
// Name/Icon/Description — see the comment on categoryMetadata in
// api/internal/http/action.go. The api map, not this file, is what the editor
// reads at serve time.
package infrastructure

const (
	CategoryName        = "Infrastructure"
	CategoryIcon        = "server"
	CategoryDescription = "Provision and manage infrastructure as code"
)
