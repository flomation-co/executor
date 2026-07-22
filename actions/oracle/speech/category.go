package speech

// Sub-category metadata for the Speech provider under Oracle Cloud. The middle path segment
// "speech" nests every oracle/speech/<verb> action under this sub-group. The api recomputes
// display metadata from its own in-code maps at serve time (subCategoryMetadata), so these are
// for manifest completeness — the Description MUST stay byte-identical to the api's
// subCategoryMetadata entry or the palette header drifts.
const (
	CategoryName        = "Speech"
	CategoryIcon        = "microphone"
	CategoryDescription = "Oracle Cloud Speech — transcribe audio to text with transcription jobs, synthesize speech, manage custom vocabularies, and list available voices"
)
