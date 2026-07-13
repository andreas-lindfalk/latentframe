package media

// Segment is a time-stamped span of transcribed narration from the walkthrough
// video. The spoken context is the raw material stage 2 (UNDERSTAND) turns into a
// per-room restage brief.
type Segment struct {
	StartMs int64  `json:"start_ms"`
	EndMs   int64  `json:"end_ms"`
	Text    string `json:"text"`
}
