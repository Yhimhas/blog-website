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
func TestCollectionAndTrackSeparation(t *testing.T) {
	fav := "https://space.bilibili.com/292715089/favlist?fid=3549762089"
	if !SourceURL(fav, "bilibili") || OfficialURL(fav, "bilibili", false) {
		t.Fatal("collection boundary")
	}
	for _, s := range []string{"https://space.bilibili.com/a/favlist?fid=1", "https://space.bilibili.com/1/favlist?fid=bad", "https://space.bilibili.com.evil/1/favlist?fid=1"} {
		if SourceURL(s, "bilibili") {
			t.Fatal(s)
		}
	}
	if OfficialURL("https://www.bilibili.com/video/not-a-bvid", "bilibili", false) {
		t.Fatal("invalid BVID")
	}
}
