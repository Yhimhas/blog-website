package netease

import (
	"blog-website/backend/internal/music/playback"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCookieFileValidationRetainsFixtures(t *testing.T) {
	root := os.Getenv("MUSIC_TEST_ARTIFACT_DIR")
	if root == "" {
		t.Skip("MUSIC_TEST_ARTIFACT_DIR required; fixture files must be retained")
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, body string
		valid      bool
	}{
		{"valid.fixture", "MUSIC_U=TEST_ONLY_NOT_A_REAL_ACCOUNT; __csrf=TEST_ONLY", true},
		{"empty.fixture", "", false},
		{"missing-auth.fixture", "other=TEST_ONLY", false},
		{"multiline.fixture", "MUSIC_U=TEST_ONLY\nAuthorization: TEST_ONLY", false},
		{"oversize.fixture", "MUSIC_U=" + strings.Repeat("x", 16385), false},
		{"control.fixture", "MUSIC_U=TEST_ONLY\x00", false},
	} {
		path, err := filepath.Abs(filepath.Join(root, test.name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(test.body), 0600); err != nil {
			t.Fatal(err)
		}
		credential, err := (CookieFile{Path: path}).Read()
		if test.valid {
			if err != nil || credential.Cookie != test.body || len(credential.Version) != 64 {
				t.Fatal("valid private file rejected", err)
			}
		} else if err != playback.SourceAuthExpired {
			t.Fatal("invalid private file accepted", test.name)
		}
	}
	if runtime.GOOS != "windows" {
		path := filepath.Join(root, "world-readable.fixture")
		if err := os.WriteFile(path, []byte("MUSIC_U=TEST_ONLY"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := (CookieFile{Path: path}).Read(); err != playback.SourceAuthExpired {
			t.Fatal("world-readable cookie file accepted")
		}
	}
	t.Log("Retained fake credential fixtures:", root)
}
