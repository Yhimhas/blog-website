package playback

// Positions are relative to the available stream, including for preview clips.
// Track metadata bounds unknown streams but does not prove their completeness.
func seekDuration(c Capability) float64 {
	if c.MediaKind == "preview" {
		if c.PreviewStart == nil || c.PreviewEnd == nil {
			return 0
		}
		d := *c.PreviewEnd - *c.PreviewStart
		if c.StreamDuration != nil && *c.StreamDuration < d {
			d = *c.StreamDuration
		}
		return float64(d)
	}
	if c.StreamDuration != nil {
		return float64(*c.StreamDuration)
	}
	if c.TrackDuration != nil {
		return float64(*c.TrackDuration)
	}
	return 0
}
