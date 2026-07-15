// Package image_common holds the category metadata and shared ImageMagick helpers
// for the image actions. No Execute function, so the manifest generator treats it
// as a category helper rather than an action.
package image_common

const (
	CategoryName        = "Image"
	CategoryIcon        = "image"
	CategoryDescription = "Process images with ImageMagick: resize, convert, crop and info"
)
