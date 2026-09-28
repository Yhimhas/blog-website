package music

import (
	"os"
	"strings"
	"testing"
)

func TestLegacySnapshot(t *testing.T) {
	f, e := os.Open("../../../frontend/src/data/bilibili-playlist.json")
	if e != nil {
		t.Skip("frontend snapshot not present in backend-only validation copy")
	}
	defer f.Close()
	s, tracks, e := DecodeBilibiliSnapshot(f)
	if e != nil || s.ID != "bilibili:3549762089" || len(tracks) != 53 {
		t.Fatal(s.ID, len(tracks), e)
	}
	for _, v := range tracks {
		if v.Availability != "unknown" || v.PartID == nil || *v.PartID != "1" || !strings.HasSuffix(v.ID, ":1") {
			t.Fatal(v)
		}
	}
	if _, _, e := DecodeBilibiliSnapshot(strings.NewReader(`{"platform":"bilibili","tracks":[]}`)); e == nil {
		t.Fatal("empty accepted")
	}
}
