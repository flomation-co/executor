// Package video_common holds the category metadata and shared ffmpeg/ffprobe
// helpers for the video (and audio) actions. It has no Execute function, so the
// manifest generator treats it as a category helper, not an action.
package video_common

const (
	CategoryName        = "Video"
	CategoryIcon        = "film"
	CategoryDescription = "Process audio and video with ffmpeg: extract audio, thumbnails, trimming and info"
)
