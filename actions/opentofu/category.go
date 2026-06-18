// Package opentofu groups the OpenTofu (Terraform-compatible) infrastructure
// actions: plan, apply, and destroy. The actual sub-actions live in the
// plan/, apply/, and destroy/ subdirectories; this file only carries the
// category metadata that the manifest generator surfaces to the editor UI.
package opentofu

const (
	CategoryName        = "OpenTofu"
	CategoryIcon        = "box+gears"
	CategoryDescription = "Infrastructure as Code — run OpenTofu (Terraform-compatible) plan, apply, and destroy against a remote backend."
)
