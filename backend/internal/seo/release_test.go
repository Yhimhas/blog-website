package seo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

const testOrigin = "https://example.test"
const testSource = "11111111-2222-3333-4444-555555555555"

func retainedDir(t *testing.T) string {
	t.Helper()
	base := os.Getenv("SEO_TEST_OUTPUT")
	if base != "" {
		if err := os.MkdirAll(base, 0755); err != nil {
			t.Fatal(err)
		}
	}
	path, err := os.MkdirTemp(base, "seo-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("Retained SEO test directory:", path)
	return path // Intentionally no cleanup or t.TempDir under the project policy.
}

func writeFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func fixtureRelease(t *testing.T, path, revision, source string) {
	t.Helper()
	m := Manifest{Revision: revision, SourceID: source, Origin: testOrigin, Files: []string{"index.html", "blog/index.html", "404.html", "sitemap.xml", "robots.txt", "assets/app-" + revision + ".js"}}
	for _, file := range m.Files {
		writeFixture(t, filepath.Join(path, file), "fixture "+revision+" "+file)
	}
	content, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(path, ManifestName), string(content))
}

func TestManifestRejectsIncompletePreviewAndUnsafePaths(t *testing.T) {
	directory := retainedDir(t)
	fixtureRelease(t, directory, "12", testSource)
	valid, err := ReadManifest(directory, testOrigin)
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*Manifest){
		func(m *Manifest) { m.Preview = true },
		func(m *Manifest) { m.Origin = "https://other.test" },
		func(m *Manifest) { m.Revision = "0" },
		func(m *Manifest) { m.SourceID = "" },
		func(m *Manifest) { m.Files = append(m.Files, "missing.html") },
		func(m *Manifest) { m.Files = append(m.Files, "../outside") },
		func(m *Manifest) { m.Files = append(m.Files, "index.html") },
		func(m *Manifest) { m.Files = []string{"index.html"} },
	} {
		m := valid
		change(&m)
		content, _ := json.Marshal(m)
		writeFixture(t, filepath.Join(directory, ManifestName), string(content))
		if _, err := ReadManifest(directory, testOrigin); err == nil {
			t.Fatalf("accepted invalid manifest: %+v", m)
		}
	}
}

func TestImmutableAssetsRetainOldChunksAndRejectCollision(t *testing.T) {
	base := retainedDir(t)
	layout := Layout{Assets: filepath.Join(base, "assets")}
	old, next := filepath.Join(base, "old"), filepath.Join(base, "next")
	writeFixture(t, filepath.Join(old, "assets/old-hash.js"), "old")
	writeFixture(t, filepath.Join(next, "assets/new-hash.js"), "new")
	for _, release := range []string{old, next, next} {
		if err := layout.PublishAssets(release); err != nil {
			t.Fatal(err)
		}
	}
	for name, want := range map[string]string{"old-hash.js": "old", "new-hash.js": "new"} {
		value, err := os.ReadFile(filepath.Join(layout.Assets, name))
		if err != nil || string(value) != want {
			t.Fatal("lost immutable chunk", name, err)
		}
	}
	writeFixture(t, filepath.Join(next, "assets/old-hash.js"), "collision")
	if err := layout.PublishAssets(next); err == nil {
		t.Fatal("overwrote immutable chunk")
	}
	value, _ := os.ReadFile(filepath.Join(layout.Assets, "old-hash.js"))
	if string(value) != "old" {
		t.Fatal("collision damaged existing asset")
	}
}

func TestLayoutPreservesExistingDirectoryAndRejectsOverlap(t *testing.T) {
	base := retainedDir(t)
	layout := Layout{Releases: filepath.Join(base, "releases"), Current: filepath.Join(base, "current"), Assets: filepath.Join(base, "assets")}
	if _, err := layout.Validate(); err != nil {
		t.Fatal(err)
	}
	unsafe := layout
	unsafe.Assets = filepath.Join(layout.Releases, "assets")
	if _, err := unsafe.Validate(); err == nil {
		t.Fatal("accepted overlapping paths")
	}
	writeFixture(t, filepath.Join(layout.Current, "keep.html"), "preserve")
	if _, err := layout.Validate(); err == nil {
		t.Fatal("accepted regular current directory")
	}
	if content, err := os.ReadFile(filepath.Join(layout.Current, "keep.html")); err != nil || string(content) != "preserve" {
		t.Fatal("existing directory damaged", err)
	}
}

func TestLinuxSwitchReplacesWholeTreeAndRetainsOldRelease(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("atomic live switching is Linux-only")
	}
	base := retainedDir(t)
	layout := Layout{Releases: filepath.Join(base, "releases"), Current: filepath.Join(base, "current"), Assets: filepath.Join(base, "assets")}
	old, next := filepath.Join(layout.Releases, "old"), filepath.Join(layout.Releases, "next")
	fixtureRelease(t, old, "1", testSource)
	fixtureRelease(t, next, "2", testSource)
	writeFixture(t, filepath.Join(old, "blog/archived/index.html"), "old article")
	for _, release := range []string{old, next} {
		if err := layout.Switch(release); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(layout.Current, "blog/archived/index.html")); !os.IsNotExist(err) {
		t.Fatal("archive leaked into current tree", err)
	}
	if _, err := os.Stat(filepath.Join(old, "blog/archived/index.html")); err != nil {
		t.Fatal("old release removed", err)
	}
	if err := layout.Switch(base); err == nil {
		t.Fatal("accepted release outside store")
	}
}

func TestLayoutRejectsAliasedDirectoryOverlap(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("directory alias validation uses Linux symlinks")
	}
	base := retainedDir(t)
	releases := filepath.Join(base, "releases")
	if err := os.Mkdir(releases, 0755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(base, "alias")
	if err := os.Symlink(releases, alias); err != nil {
		t.Fatal(err)
	}
	layout := Layout{Releases: releases, Current: filepath.Join(base, "current"), Assets: filepath.Join(alias, "assets")}
	if _, err := layout.Validate(); err == nil {
		t.Fatal("accepted alias into release tree")
	}
}
