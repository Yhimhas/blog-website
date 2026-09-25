package blog

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPatchOmissionAndNull(t *testing.T) {
	category := "cat"
	p := Record{Slug: "hello", Title: "标题", Summary: "original", CategoryID: &category, TagIDs: []string{"tag"}}
	if err := ApplyPatch(&p, map[string]json.RawMessage{"summary": json.RawMessage(`""`), "categoryId": json.RawMessage(`null`), "tagIds": json.RawMessage(`[]`)}); err != nil {
		t.Fatal(err)
	}
	if p.Title != "标题" || p.Summary != "" || p.CategoryID != nil || len(p.TagIDs) != 0 {
		t.Fatalf("patch semantics: %+v", p)
	}
	for _, field := range []string{"title", "summary", "tagIds", "contentMarkdown"} {
		if ApplyPatch(&p, map[string]json.RawMessage{field: json.RawMessage(`null`)}) == nil {
			t.Errorf("accepted null %s", field)
		}
	}
	if ApplyPatch(&p, map[string]json.RawMessage{"status": json.RawMessage(`"published"`)}) == nil {
		t.Fatal("accepted privileged field")
	}
}
func TestRecordBounds(t *testing.T) {
	p := Record{Slug: "valid-slug", Title: strings.Repeat("中", 160), ContentMarkdown: strings.Repeat("x", 200*1024)}
	if !ValidRecord(p) {
		t.Fatal("boundary rejected")
	}
	p.Title += "中"
	if ValidRecord(p) {
		t.Fatal("overlong title accepted")
	}
	p.Title = "ok"
	p.ContentMarkdown += "x"
	if ValidRecord(p) {
		t.Fatal("overlong body accepted")
	}
}
func TestLiteralSearchEscapes(t *testing.T) {
	if got := escapedSearch(`a%_\`); got != `%a\%\_\\%` {
		t.Fatal(got)
	}
}
