package netease

import (
	"blog-website/backend/internal/music/provider"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

var testRef = provider.Ref{Provider: "netease", ExternalID: "123", SourceURL: "https://music.163.com/playlist?id=123"}

func metadataResponse(body string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/plain; charset=UTF-8"}}, Body: io.NopCloser(strings.NewReader(body))}
}

func detailFixture(ids []string, preview int) string {
	list := struct {
		TrackCount int      `json:"trackCount"`
		TrackIDs   []songID `json:"trackIds"`
		Tracks     []song   `json:"tracks"`
	}{TrackCount: len(ids), TrackIDs: []songID{}, Tracks: []song{}}
	for i, id := range ids {
		list.TrackIDs = append(list.TrackIDs, songID{ID: json.Number(id)})
		if i < preview {
			list.Tracks = append(list.Tracks, song{ID: json.Number(id), Name: "song " + id})
		}
	}
	encoded, _ := json.Marshal(map[string]any{"code": 200, "playlist": list})
	return string(encoded)
}

func songsForRequest(t *testing.T, req *http.Request) string {
	t.Helper()
	var ids []songID
	if err := json.Unmarshal([]byte(req.URL.Query().Get("c")), &ids); err != nil || len(ids) == 0 || len(ids) > detailBatchSize {
		t.Fatalf("invalid batch: %s", req.URL.Query().Get("c"))
	}
	songs := []song{}
	// Return reverse order to exercise reconstruction, not upstream ordering.
	for i := len(ids) - 1; i >= 0; i-- {
		songs = append(songs, song{ID: ids[i].ID, Name: "song " + ids[i].ID.String()})
	}
	encoded, _ := json.Marshal(map[string]any{"code": 200, "songs": songs})
	return string(encoded)
}

func TestFetchCompletes265TracksInBoundedBatches(t *testing.T) {
	ids := []string{}
	for i := 1; i <= 265; i++ {
		ids = append(ids, strconv.Itoa(i))
	}
	details, batches, fetched := 0, 0, 0
	c := &Client{HTTP: &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Scheme != "https" || req.URL.Host != "music.163.com" || req.Header.Get("Cookie") != "" || req.Method != "GET" {
			t.Fatal("unsafe metadata request")
		}
		switch req.URL.Path {
		case "/api/v6/playlist/detail":
			details++
			if req.URL.Query().Get("n") != "2000" || req.URL.Query().Get("id") != testRef.ExternalID {
				t.Fatal("unbounded detail request")
			}
			return metadataResponse(detailFixture(ids, 6)), nil
		case "/api/v3/song/detail":
			batches++
			var batch []songID
			if json.Unmarshal([]byte(req.URL.Query().Get("c")), &batch) != nil {
				t.Fatal("invalid batch JSON")
			}
			for _, id := range batch {
				if n, _ := strconv.Atoi(id.ID.String()); n <= 6 {
					t.Fatal("requested already complete preview")
				}
			}
			fetched += len(batch)
			return metadataResponse(songsForRequest(t, req)), nil
		default:
			t.Fatalf("unexpected endpoint %s", req.URL.Path)
			return nil, nil
		}
	})}}
	tracks, err := c.FetchPlaylist(context.Background(), testRef)
	if err != nil || len(tracks) != 265 || details != 2 || batches != 6 || fetched != 259 {
		t.Fatalf("got tracks=%d details=%d batches=%d fetched=%d err=%v", len(tracks), details, batches, fetched, err)
	}
	for i, track := range tracks {
		if track.ExternalID != ids[i] || track.Availability != "unknown" {
			t.Fatalf("order/availability mismatch at %d: %+v", i, track)
		}
	}
}

func TestCompletionPreservesLargeIDsAndFirstDuplicatePosition(t *testing.T) {
	ids := []string{"900719925474099312345678901234", "2", "900719925474099312345678901234", "1"}
	requests := 0
	c := &Client{HTTP: &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.URL.Path == "/api/v6/playlist/detail" {
			return metadataResponse(detailFixture(ids, 0)), nil
		}
		var batch []songID
		if json.Unmarshal([]byte(req.URL.Query().Get("c")), &batch) != nil || len(batch) != 3 || batch[0].ID.String() != ids[0] {
			t.Fatal("batch lost ID precision or repeated an ID")
		}
		return metadataResponse(songsForRequest(t, req)), nil
	})}}
	tracks, err := c.FetchPlaylist(context.Background(), testRef)
	if err != nil || len(tracks) != 3 || requests != 3 {
		t.Fatalf("tracks=%v requests=%d error=%v", tracks, requests, err)
	}
	if got := []string{tracks[0].ExternalID, tracks[1].ExternalID, tracks[2].ExternalID}; !reflect.DeepEqual(got, []string{ids[0], "2", "1"}) {
		t.Fatal("original order lost", got)
	}
}

func TestMalformedIDListsNeverTriggerCompletion(t *testing.T) {
	for _, tc := range []struct{ name, body, code string }{
		{"missing IDs", `{"code":200,"playlist":{"trackCount":2,"tracks":[]}}`, "INCOMPLETE_PLAYLIST"},
		{"partial IDs", `{"code":200,"playlist":{"trackCount":2,"trackIds":[{"id":1}],"tracks":[]}}`, "INCOMPLETE_PLAYLIST"},
		{"missing count", `{"code":200,"playlist":{"trackIds":[{"id":1}],"tracks":[]}}`, "INVALID_PAYLOAD"},
		{"malformed ID", `{"code":200,"playlist":{"trackCount":1,"trackIds":[{"id":1.5}],"tracks":[]}}`, "INVALID_PAYLOAD"},
		{"missing ID", `{"code":200,"playlist":{"trackCount":1,"trackIds":[{}],"tracks":[]}}`, "INVALID_PAYLOAD"},
		{"oversized ID", `{"code":200,"playlist":{"trackCount":1,"trackIds":[{"id":1234567890123456789012345678901}],"tracks":[]}}`, "INVALID_PAYLOAD"},
		{"foreign preview", `{"code":200,"playlist":{"trackCount":1,"trackIds":[{"id":1}],"tracks":[{"id":2,"name":"foreign"}]}}`, "INVALID_PAYLOAD"},
		{"invalid preview", `{"code":200,"playlist":{"trackCount":1,"trackIds":[{"id":1}],"tracks":[{"id":1,"name":""}]}}`, "INVALID_PAYLOAD"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			c := &Client{HTTP: &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
				requests++
				return metadataResponse(tc.body), nil
			})}}
			tracks, err := c.FetchPlaylist(context.Background(), testRef)
			if tracks != nil || provider.Code(err) != tc.code || requests != 1 {
				t.Fatalf("tracks=%v error=%v requests=%d", tracks, err, requests)
			}
		})
	}
}

func TestBatchFailureReturnsNoPartialCandidate(t *testing.T) {
	for _, tc := range []struct {
		name, body, code string
		status           int
	}{
		{"missing song", `{"code":200,"songs":[]}`, "INCOMPLETE_PLAYLIST", 200},
		{"foreign song", `{"code":200,"songs":[{"id":99,"name":"foreign"}]}`, "INVALID_PAYLOAD", 200},
		{"conflicting duplicate", `{"code":200,"songs":[{"id":2,"name":"a"},{"id":2,"name":"b"}]}`, "INVALID_PAYLOAD", 200},
		{"missing field", `{"code":200}`, "INVALID_PAYLOAD", 200},
		{"null songs", `{"code":200,"songs":null}`, "INVALID_PAYLOAD", 200},
		{"bad title", `{"code":200,"songs":[{"id":2,"name":""}]}`, "INVALID_PAYLOAD", 200},
		{"refused", `{"code":20001,"songs":false}`, "UPSTREAM_ACCESS_RESTRICTED", 200},
		{"unauthorized", `{"code":403}`, "UPSTREAM_UNAUTHORIZED", 200},
		{"business limit", `{"code":429}`, "UPSTREAM_RATE_LIMITED", 200},
		{"HTTP limit", `{"code":200}`, "UPSTREAM_RATE_LIMITED", 429},
		{"unknown code", `{"code":50001}`, "UPSTREAM_REJECTED", 200},
		{"invalid JSON", `<html>unavailable</html>`, "INVALID_PAYLOAD", 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			c := &Client{HTTP: &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				if requests == 1 {
					return metadataResponse(detailFixture([]string{"1", "2"}, 1)), nil
				}
				r := metadataResponse(tc.body)
				r.StatusCode = tc.status
				return r, nil
			})}}
			tracks, err := c.FetchPlaylist(context.Background(), testRef)
			if tracks != nil || provider.Code(err) != tc.code || requests != 2 {
				t.Fatalf("tracks=%v error=%v requests=%d", tracks, err, requests)
			}
		})
	}
}

func TestRecheckRejectsPlaylistChangesAndDenial(t *testing.T) {
	for _, latest := range []string{
		detailFixture([]string{"2", "1"}, 0), // Same count, changed order.
		detailFixture([]string{"1", "3"}, 0), // Same count, replaced song.
		detailFixture([]string{"1", "2", "3"}, 0),
		`{"code":20001}`,
	} {
		requests := 0
		c := &Client{HTTP: &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
			requests++
			if requests == 1 {
				return metadataResponse(detailFixture([]string{"1", "2"}, 1)), nil
			}
			if req.URL.Path == "/api/v3/song/detail" {
				return metadataResponse(songsForRequest(t, req)), nil
			}
			return metadataResponse(latest), nil
		})}}
		tracks, err := c.FetchPlaylist(context.Background(), testRef)
		want := "INCOMPLETE_PLAYLIST"
		if latest == `{"code":20001}` {
			want = "UPSTREAM_ACCESS_RESTRICTED"
		}
		if tracks != nil || provider.Code(err) != want || requests != 3 {
			t.Fatalf("tracks=%v error=%v requests=%d", tracks, err, requests)
		}
	}
}

func TestCompleteAndEmptySnapshotsAvoidExtraRequests(t *testing.T) {
	for _, ids := range [][]string{{"2", "1"}, {}} {
		requests := 0
		c := &Client{HTTP: &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
			requests++
			return metadataResponse(detailFixture(ids, len(ids))), nil
		})}}
		tracks, err := c.FetchPlaylist(context.Background(), testRef)
		if err != nil || tracks == nil || len(tracks) != len(ids) || requests != 1 {
			t.Fatalf("tracks=%v error=%v requests=%d", tracks, err, requests)
		}
	}
}

func TestParseReordersCompleteTracksByDeclaredIDs(t *testing.T) {
	tracks, err := Parse([]byte(`{"code":200,"playlist":{"trackCount":2,"trackIds":[{"id":2},{"id":1}],"tracks":[{"id":1,"name":"one"},{"id":2,"name":"two"}]}}`))
	if err != nil || len(tracks) != 2 || tracks[0].ExternalID != "2" || tracks[1].ExternalID != "1" {
		t.Fatalf("declared order lost: tracks=%v error=%v", tracks, err)
	}
}

func TestBatchTimeoutUsesOverallContextAndStopsRequests(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	requests := 0
	c := &Client{HTTP: &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if requests == 1 {
			return metadataResponse(detailFixture([]string{"1", "2"}, 1)), nil
		}
		<-req.Context().Done()
		return nil, req.Context().Err()
	})}}
	tracks, err := c.FetchPlaylist(ctx, testRef)
	if tracks != nil || provider.Code(err) != "UPSTREAM_TIMEOUT" || requests != 2 {
		t.Fatalf("tracks=%v error=%v requests=%d", tracks, err, requests)
	}
}

func TestLaterBatchFailureDiscardsEarlierSuccessfulDetails(t *testing.T) {
	ids := []string{}
	for i := 1; i <= 60; i++ {
		ids = append(ids, strconv.Itoa(i))
	}
	requests := 0
	c := &Client{HTTP: &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if requests == 1 {
			return metadataResponse(detailFixture(ids, 1)), nil
		}
		if requests == 2 {
			return metadataResponse(songsForRequest(t, req)), nil
		}
		return metadataResponse(`{"code":429}`), nil
	})}}
	tracks, err := c.FetchPlaylist(context.Background(), testRef)
	if tracks != nil || provider.Code(err) != "UPSTREAM_RATE_LIMITED" || requests != 3 {
		t.Fatalf("tracks=%v error=%v requests=%d", tracks, err, requests)
	}
}

func TestSourceMismatchMakesNoRequest(t *testing.T) {
	c := &Client{HTTP: &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("requested invalid source")
		return nil, nil
	})}}
	ref := testRef
	ref.SourceURL = "https://music.163.com/playlist?id=999"
	if _, err := c.FetchPlaylist(context.Background(), ref); err != provider.InvalidSource {
		t.Fatal(err)
	}
}

// Explicit opt-in: reads upstream metadata only, never opens a database or saves media.
func TestLivePlaylistCompletion(t *testing.T) {
	id := os.Getenv("NETEASE_LIVE_PLAYLIST_ID")
	if id == "" {
		t.Skip("NETEASE_LIVE_PLAYLIST_ID not configured; upstream validation not executed")
	}
	if !provider.Decimal.MatchString(id) {
		t.Fatal("invalid live playlist ID")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	started := time.Now()
	client := New()
	defer client.HTTP.CloseIdleConnections()
	ref := provider.Ref{Provider: "netease", ExternalID: id, SourceURL: "https://music.163.com/playlist?id=" + id}
	tracks, err := client.FetchPlaylist(provider.WithSyncLogger(ctx, provider.SyncLogger(ctx).With("sourceId", "netease:"+id, "requestId", "read-only-live-validation")), ref)
	if err != nil {
		t.Fatalf("live completion failed: %s", provider.Code(err))
	}
	if err := provider.Validate(tracks, "netease"); err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, track := range tracks {
		if track.Availability != "unknown" {
			t.Fatal("metadata incorrectly establishes playback availability")
		}
		ids = append(ids, track.ExternalID)
	}
	checksum := sha256.Sum256([]byte(strings.Join(ids, ",")))
	t.Log(fmt.Sprintf("read-only complete source=netease:%s tracks=%d orderSHA256=%x elapsed=%s; no snapshot written", id, len(tracks), checksum, time.Since(started)))
}
