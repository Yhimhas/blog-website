package music

import (
	"blog-website/backend/internal/music/provider"
	"blog-website/backend/internal/music/provider/netease"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type refusedPlaylist struct{ body string }

func (a refusedPlaylist) FetchPlaylist(context.Context, provider.Ref) ([]provider.Track, error) {
	return netease.Parse([]byte(a.body))
}

type syncMetadataTransport func(*http.Request) (*http.Response, error)

func (f syncMetadataTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func incompleteBatchAdapter(songBody string, changed bool) provider.Adapter {
	details := 0
	return &netease.Client{HTTP: &http.Client{Transport: syncMetadataTransport(func(req *http.Request) (*http.Response, error) {
		body := songBody
		if req.URL.Path == "/api/v6/playlist/detail" {
			details++
			body = `{"code":200,"playlist":{"trackCount":2,"trackIds":[{"id":1},{"id":2}],"tracks":[{"id":1,"name":"preview"}]}}`
			if changed && details > 1 {
				body = `{"code":200,"playlist":{"trackCount":2,"trackIds":[{"id":2},{"id":1}],"tracks":[]}}`
			}
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/plain"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}}
}

// A scripted SQL connection exercises the actual GORM updates without connecting
// to a database. Any extra write (including snapshot replacement) fails the test.
type syncSQLStep struct {
	op, contains string
	columns      []string
	values       []driver.Value
	check        func(string, []driver.NamedValue)
}

type syncSQL struct {
	t     *testing.T
	steps []syncSQLStep
}

func (s *syncSQL) next(op, query string, args []driver.NamedValue) (syncSQLStep, error) {
	s.t.Helper()
	if len(s.steps) == 0 {
		s.t.Errorf("unexpected %s: %s", op, query)
		return syncSQLStep{}, fmt.Errorf("unexpected SQL operation")
	}
	step := s.steps[0]
	s.steps = s.steps[1:]
	if step.op != op || !strings.Contains(query, step.contains) {
		s.t.Errorf("got %s %q, want %s containing %q", op, query, step.op, step.contains)
		return step, fmt.Errorf("unexpected SQL operation")
	}
	if step.check != nil {
		step.check(query, args)
	}
	return step, nil
}

func (s *syncSQL) Connect(context.Context) (driver.Conn, error) { return s, nil }
func (s *syncSQL) Driver() driver.Driver                        { return s }
func (s *syncSQL) Open(string) (driver.Conn, error)             { return s, nil }
func (s *syncSQL) Close() error                                 { return nil }
func (s *syncSQL) Prepare(string) (driver.Stmt, error) {
	return nil, fmt.Errorf("prepared statements are not expected")
}
func (s *syncSQL) Begin() (driver.Tx, error) {
	_, err := s.next("begin", "", nil)
	return s, err
}
func (s *syncSQL) Commit() error   { _, err := s.next("commit", "", nil); return err }
func (s *syncSQL) Rollback() error { _, err := s.next("rollback", "", nil); return err }
func (s *syncSQL) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	step, err := s.next("query", q, args)
	return &syncSQLRows{columns: step.columns, values: step.values}, err
}
func (s *syncSQL) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	_, err := s.next("exec", q, args)
	return driver.RowsAffected(1), err
}

type syncSQLRows struct {
	columns []string
	values  []driver.Value
}

func (r *syncSQLRows) Columns() []string { return r.columns }
func (r *syncSQLRows) Close() error      { return nil }
func (r *syncSQLRows) Next(out []driver.Value) error {
	if r.values == nil {
		return io.EOF
	}
	copy(out, r.values)
	r.values = nil
	return nil
}

func TestSyncFailurePreservesSnapshotAndExposesError(t *testing.T) {
	for _, tc := range []struct {
		body, code string
		adapter    provider.Adapter
	}{
		{body: `{"msg":"","code":20001}`, code: "UPSTREAM_ACCESS_RESTRICTED"},
		{body: `{"code":50001}`, code: "UPSTREAM_REJECTED"},
		{body: `{"code":200,"playlist":{"trackCount":265,"tracks":[]}}`, code: "INCOMPLETE_PLAYLIST"},
		{body: `{"code":200,"playlist":false}`, code: "INVALID_PAYLOAD"},
		{code: "INCOMPLETE_PLAYLIST", adapter: incompleteBatchAdapter(`{"code":200,"songs":[]}`, false)},
		{code: "UPSTREAM_RATE_LIMITED", adapter: incompleteBatchAdapter(`{"code":429}`, false)},
		{code: "INCOMPLETE_PLAYLIST", adapter: incompleteBatchAdapter(`{"code":200,"songs":[{"id":2,"name":"detail"}]}`, true)},
	} {
		t.Run(tc.code, func(t *testing.T) {
			oldTime := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)
			source := Source{ID: "netease:123", Provider: "netease", ExternalID: "123", Title: "retained", SourceURL: "https://music.163.com/playlist?id=123", SyncStatus: "running", SyncedAt: &oldTime, SnapshotHash: "old-hash"}
			columns := []string{"id", "provider", "external_id", "title", "source_url", "sync_status", "synced_at", "snapshot_hash", "last_error_code", "track_count"}
			values := []driver.Value{source.ID, source.Provider, source.ExternalID, source.Title, source.SourceURL, "failed", oldTime, "old-hash", tc.code, int64(2)}
			checkCode := func(q string, args []driver.NamedValue) {
				found := false
				for _, arg := range args {
					if arg.Value == tc.code {
						found = true
					}
				}
				if !found {
					t.Errorf("update lost error classification: %s", q)
				}
			}
			connection := &syncSQL{t: t, steps: []syncSQLStep{
				{op: "begin"},
				{op: "query", contains: `FROM "music_sources"`, columns: columns, values: values},
				{op: "exec", contains: `INSERT INTO "netease_sync_runs"`},
				{op: "exec", contains: `UPDATE "music_sources"`, check: func(q string, args []driver.NamedValue) {
					if !strings.Contains(q, `"last_error_code"=`) || !strings.Contains(q, `"last_error_at"=`) {
						t.Errorf("retry did not clear stale failure metadata: %s", q)
					}
					for _, arg := range args {
						if arg.Value == tc.code {
							t.Error("retry kept the stale error code")
						}
					}
				}},
				{op: "commit"},
				{op: "begin"},
				{op: "query", contains: `FROM "music_sources"`, columns: columns, values: values},
				{op: "exec", contains: `UPDATE "netease_sync_runs"`, check: checkCode},
				{op: "exec", contains: `UPDATE "music_sources"`, check: func(q string, args []driver.NamedValue) {
					checkCode(q, args)
					// Permit only failure metadata; no saved snapshot fields can change.
					for _, field := range []string{"synced_at", "snapshot_hash", "source_items", "music_items", "track_count"} {
						if strings.Contains(q, field) {
							t.Errorf("failure writes snapshot field %s: %s", field, q)
						}
					}
					if !strings.Contains(q, `"last_error_code"=`) {
						t.Errorf("error code column was not written: %s", q)
					}
				}},
				{op: "commit"},
				{op: "query", contains: "AS track_count", columns: columns, values: values},
			}}
			pool := sql.OpenDB(connection)
			defer pool.Close()
			db, err := gorm.Open(postgres.New(postgres.Config{Conn: pool}), &gorm.Config{DisableAutomaticPing: true})
			if err != nil {
				t.Fatal(err)
			}
			adapter := tc.adapter
			if adapter == nil {
				adapter = refusedPlaylist{tc.body}
			}
			service := New(db, adapter, context.Background())
			if _, err := service.StartSync(context.Background(), source.ID, "local-fixture"); err != nil {
				t.Fatal(err)
			}
			service.Stop()
			playlists, err := service.Playlists(context.Background())
			if err != nil || len(playlists) != 1 {
				t.Fatalf("public playlists: %v %v", playlists, err)
			}
			got := playlists[0]
			if got.SyncErrorCode == nil || *got.SyncErrorCode != tc.code || got.SyncStatus != "failed" || got.TrackCount != 2 || got.SnapshotHash != "old-hash" || got.SyncedAt == nil || !got.SyncedAt.Equal(oldTime) {
				t.Fatalf("failure status or retained metadata lost: %+v", got)
			}
			encoded, err := json.Marshal(got)
			if err != nil || !strings.Contains(string(encoded), `"syncErrorCode":"`+tc.code+`"`) || strings.Contains(string(encoded), "snapshotHash") || strings.Contains(string(encoded), "last_error_code") {
				t.Fatalf("public DTO: %s %v", encoded, err)
			}
			if len(connection.steps) != 0 {
				t.Fatalf("%d SQL operations not performed", len(connection.steps))
			}
		})
	}
}
