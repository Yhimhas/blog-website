package playback

import "strings"

// DetectFormat accepts only audio/container signatures understood by the bounded
// stream pipeline. HTML, JSON, playlists and arbitrary input formats stay closed.
func DetectFormat(b []byte, contentType string) string {
	ct := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	if ct != "" && ct != "application/octet-stream" && ct != "binary/octet-stream" && ct != "application/ogg" && ct != "video/mp4" && ct != "video/webm" && !strings.HasPrefix(ct, "audio/") {
		return ""
	}
	if len(b) < 12 {
		return ""
	}
	switch {
	case string(b[4:8]) == "ftyp":
		return "mov"
	case string(b[:4]) == "fLaC":
		return "flac"
	case string(b[:4]) == "OggS":
		return "ogg"
	case string(b[:4]) == "RIFF" && string(b[8:12]) == "WAVE":
		return "wav"
	case b[0] == 0x1a && b[1] == 0x45 && b[2] == 0xdf && b[3] == 0xa3:
		return "matroska"
	case string(b[:3]) == "ID3" && b[3] >= 2 && b[3] <= 4 && b[6]|b[7]|b[8]|b[9] < 128:
		return "mp3"
	case b[0] == 0xff && b[1]&0xf6 == 0xf0:
		return "aac"
	case b[0] == 0xff && b[1]&0xe0 == 0xe0 && b[1]&6 != 0 && b[1]&0x18 != 8 && b[2]&0xf0 != 0xf0 && b[2]&0xf0 != 0 && b[2]&12 != 12:
		return "mp3"
	}
	return ""
}
func FormatMIME(format string) string {
	switch format {
	case "mov":
		return "audio/mp4"
	case "mp3":
		return "audio/mpeg"
	case "flac":
		return "audio/flac"
	case "ogg":
		return "audio/ogg"
	case "wav":
		return "audio/wav"
	case "matroska":
		return "audio/webm"
	case "aac":
		return "audio/aac"
	}
	return ""
}
