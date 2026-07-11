// Package forms_common holds the top-level "Forms" category metadata plus the
// shared HTTP plumbing and result shapers used by every forms provider
// sub-category (e.g. Typeform). Each provider lives in a sub-directory with its
// own category.go, which the manifest generator emits as a sub-category so the
// editor renders "Forms > Provider > Action".
//
// This package intentionally has no Execute function, so the manifest generator
// treats it as a category holder (like ukgov_common / git_common) rather than
// an action.
package forms_common

const (
	CategoryName        = "Forms"
	CategoryIcon        = "clipboard-list"
	CategoryDescription = "Create forms, collect responses and trigger flows from external form providers"
)
