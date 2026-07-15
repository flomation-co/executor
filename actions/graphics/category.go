// Package graphics_common holds the category metadata and the shared Go (gg) frame
// renderer for the animated-graphics actions. Text is rendered in pure Go with
// EMBEDDED fonts (no system fonts, no libfreetype) for deterministic output; ffmpeg
// only assembles the frames into a transparent video. No Execute here.
package graphics_common

const (
	CategoryName        = "Graphics"
	CategoryIcon        = "pen"
	CategoryDescription = "Generate animated graphics — titles, lower-thirds and counters — as transparent overlays"
)
