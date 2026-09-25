package provider

import "testing"

func TestOfficialURL(t *testing.T) {
	for _, raw := range []string{"http://music.163.com/playlist?id=1", "https://music.163.com.evil.test/playlist?id=1", "https://user@music.163.com/playlist?id=1", "https://127.0.0.1/playlist?id=1", "https://music.163.com:443/playlist?id=1", "https://music.163.com/redirect"} {
		if OfficialURL(raw, "netease", false) {
			t.Errorf("accepted %s", raw)
		}
	}
	if !OfficialURL("https://music.163.com/outchain/player?id=123&type=0", "netease", true) {
		t.Fatal("official embed rejected")
	}
}
