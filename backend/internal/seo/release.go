package seo

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

const ManifestName = "seo-release.json"

var revisionPattern = regexp.MustCompile(`^[1-9][0-9]*$`)
var sourcePattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type Manifest struct {
	Revision string   `json:"revision"`
	SourceID string   `json:"sourceId"`
	Origin   string   `json:"origin"`
	Preview  bool     `json:"preview"`
	Files    []string `json:"files"`
}

type Layout struct {
	Releases string
	Current  string
	Assets   string
}

// Resolve paths once and reject overlap, so a fresh output directory never
// lands inside the live root or the immutable asset store.
func (l Layout) Validate() (Layout, error) {
	paths := []*string{&l.Releases, &l.Current, &l.Assets}
	for _, path := range paths {
		if *path == "" {
			return l, errors.New("SEO_RELEASES_DIR, SEO_CURRENT_LINK and SEO_ASSETS_DIR are required")
		}
		abs, err := filepath.Abs(*path)
		if err != nil {
			return l, err
		}
		*path = filepath.Clean(abs)
	}
	// Resolve directory aliases too: different lexical paths can refer to
	// the same tree. Only current's parent is resolved, keeping its pointer.
	for _, path := range []*string{&l.Releases, &l.Assets} {
		physical, err := physicalPath(*path)
		if err != nil {
			return l, err
		}
		*path = physical
	}
	parent, err := physicalPath(filepath.Dir(l.Current))
	if err != nil {
		return l, err
	}
	l.Current = filepath.Join(parent, filepath.Base(l.Current))
	for i, a := range paths {
		for j, b := range paths {
			if i != j && within(*a, *b) {
				return l, errors.New("SEO release, current and asset paths must not overlap")
			}
		}
	}
	return l, checkCurrent(l.Current)
}

func physicalPath(path string) (string, error) {
	if _, err := os.Lstat(path); err == nil {
		return filepath.EvalSymlinks(path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	parentPath := filepath.Dir(path)
	if parentPath == path {
		return "", errors.New("SEO path has no accessible filesystem root")
	}
	parent, err := physicalPath(parentPath)
	if err != nil {
		return "", err
	}
	return filepath.Join(parent, filepath.Base(path)), nil
}

func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !filepath.IsAbs(rel) && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func checkCurrent(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return errors.New("SEO_CURRENT_LINK must be absent or a symlink; existing files/directories are preserved")
	}
	return nil
}

func uniqueID() (string, error) {
	var value [12]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

func validOrigin(origin string) bool {
	u, err := url.Parse(origin)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && u.User == nil && u.RawQuery == "" && u.Fragment == "" && (u.Path == "" || u.Path == "/")
}

func ReadManifest(directory, origin string) (Manifest, error) {
	var m Manifest
	content, err := os.ReadFile(filepath.Join(directory, ManifestName))
	if err != nil {
		return m, err
	}
	if json.Unmarshal(content, &m) != nil || !revisionPattern.MatchString(m.Revision) || !sourcePattern.MatchString(m.SourceID) || m.Origin != origin || m.Preview || !validOrigin(m.Origin) || len(m.Files) == 0 {
		return m, errors.New("invalid, preview or wrong-origin SEO release manifest")
	}
	seen := map[string]bool{}
	for _, name := range m.Files {
		if name == "" || filepath.IsAbs(name) || filepath.ToSlash(filepath.Clean(name)) != name || !within(directory, filepath.Join(directory, name)) || seen[name] {
			return m, errors.New("invalid SEO release file path")
		}
		info, err := os.Lstat(filepath.Join(directory, name))
		if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
			return m, errors.New("SEO release file missing or empty")
		}
		seen[name] = true
	}
	for _, name := range []string{"index.html", "blog/index.html", "sitemap.xml", "robots.txt", "404.html"} {
		if !seen[name] {
			return m, errors.New("SEO manifest omits required public files")
		}
	}
	return m, nil
}

// Only Linux rename semantics are supported for live switching. No release or
// regular file is removed. The temporary link is next to current (same mount).
func (l Layout) Switch(release string) error {
	if runtime.GOOS != "linux" {
		return errors.New("atomic SEO publication requires Linux")
	}
	if !within(l.Releases, release) || filepath.Clean(release) == l.Releases {
		return errors.New("release outside SEO_RELEASES_DIR")
	}
	if err := checkCurrent(l.Current); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(l.Current), 0755); err != nil {
		return err
	}
	id, err := uniqueID()
	if err != nil {
		return err
	}
	link := l.Current + ".next-" + id
	if err := os.Symlink(release, link); err != nil {
		return err
	}
	if err := os.Rename(link, l.Current); err != nil {
		return err // Failed staging links are intentionally retained for inspection.
	}
	return syncDir(filepath.Dir(l.Current))
}

func syncDir(path string) error {
	if runtime.GOOS != "linux" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}

// Old HTML can still request old hashed chunks after a switch. Keep immutable
// assets in an independent alias, never copy old article HTML into a new root.
func (l Layout) PublishAssets(release string) error {
	root := filepath.Join(release, "assets")
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return errors.New("asset must be a regular file")
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		target := filepath.Join(l.Assets, rel)
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		existing, err := os.ReadFile(target)
		if err == nil {
			if !bytes.Equal(existing, content) {
				return errors.New("immutable asset collision; existing asset preserved")
			}
			return nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		id, err := uniqueID()
		if err != nil {
			return err
		}
		stage := target + ".pending-" + id
		f, err := os.OpenFile(stage, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		_, writeErr := f.Write(content)
		if writeErr == nil {
			writeErr = f.Sync()
		}
		closeErr := f.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
		if err := os.Rename(stage, target); err != nil {
			return err
		}
		return syncDir(filepath.Dir(target))
	})
}
