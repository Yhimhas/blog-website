package music

import (
	"blog-website/backend/internal/auth"
	"blog-website/backend/internal/music/provider"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"sync"
	"time"
)

var ErrBusy = errors.New("sync in progress")
var ErrCapacity = errors.New("sync capacity exceeded")
var errAttempted = errors.New("already attempted today")

type Source struct {
	ID           string     `json:"id"`
	Provider     string     `json:"provider"`
	ExternalID   string     `json:"-"`
	Title        string     `json:"title"`
	SourceURL    string     `json:"sourceUrl"`
	EmbedURL     *string    `json:"embedUrl"`
	Enabled      bool       `json:"-"`
	SyncStatus   string     `json:"syncStatus"`
	SyncedAt     *time.Time `json:"syncedAt"`
	SnapshotHash string     `json:"-"`
	TrackCount   int64      `json:"trackCount" gorm:"->"`
}

func (Source) TableName() string { return "music_sources" }

type Run struct {
	ID             string     `json:"runId"`
	SourceID       string     `json:"sourceId"`
	Status         string     `json:"status"`
	StartedAt      time.Time  `json:"startedAt"`
	FinishedAt     *time.Time `json:"finishedAt"`
	CandidateCount int        `json:"candidateCount"`
	ErrorCode      *string    `json:"errorCode"`
	RequestID      string     `json:"requestId"`
}

func (Run) TableName() string { return "netease_sync_runs" }

type Service struct {
	DB       *gorm.DB
	Adapter  provider.Adapter
	Context  context.Context
	mu       sync.Mutex
	stopping bool
	workers  sync.WaitGroup
	slots    chan struct{}
}

func New(db *gorm.DB, adapter provider.Adapter, ctx context.Context) *Service {
	return &Service{DB: db, Adapter: adapter, Context: ctx, slots: make(chan struct{}, 2)}
}
func (s *Service) Stop() { s.mu.Lock(); s.stopping = true; s.mu.Unlock(); s.workers.Wait() }
func (s *Service) Playlists(ctx context.Context) ([]Source, error) {
	out := []Source{}
	err := s.DB.WithContext(ctx).Table("music_sources s").Select("s.*, (SELECT count(*) FROM source_items si WHERE si.source_id=s.id AND si.active) AS track_count").Where("s.enabled").Order("s.title,s.id").Scan(&out).Error
	if err != nil {
		return nil, err
	}
	for _, v := range out {
		if !provider.OfficialURL(v.SourceURL, v.Provider, false) || (v.EmbedURL != nil && !provider.OfficialURL(*v.EmbedURL, v.Provider, true)) {
			return nil, provider.InvalidSource
		}
	}
	return out, nil
}

type trackRow struct {
	provider.Track
	PartKey string
}

func ReadTracks(db *gorm.DB) ([]provider.Track, error) {
	rows := []trackRow{}
	if err := db.Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := []provider.Track{}
	for _, row := range rows {
		v := row.Track
		if row.PartKey != "" {
			part := row.PartKey
			v.PartID = &part
		}
		if err := provider.Validate([]provider.Track{v}, v.Provider); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}
func (s *Service) Tracks(ctx context.Context, id string, page, size int) ([]provider.Track, int64, error) {
	out := []provider.Track{}
	var total int64
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var source Source
		if err := tx.Where("id=? AND enabled", id).Take(&source).Error; err != nil {
			return err
		}
		q := tx.Table("music_items m").Joins("JOIN source_items si ON si.item_id=m.id").Where("si.source_id=? AND si.active", id)
		if err := q.Count(&total).Error; err != nil {
			return err
		}
		if total == 0 || int64(page-1) > (total-1)/int64(size) {
			return nil
		}
		var err error
		out, err = ReadTracks(q.Select("m.*").Order("si.position,m.id").Offset((page - 1) * size).Limit(size))
		return err
	})
	return out, total, err
}
func (s *Service) GetRun(ctx context.Context, id string) (Run, error) {
	var run Run
	err := s.DB.WithContext(ctx).Where("id=?", id).Take(&run).Error
	return run, err
}
func (s *Service) StartSync(ctx context.Context, id, requestID string) (Run, error) {
	return s.startSync(ctx, id, requestID, time.Time{})
}
func (s *Service) startSync(ctx context.Context, id, requestID string, since time.Time) (Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopping || s.Context.Err() != nil {
		return Run{}, ErrCapacity
	}
	select {
	case s.slots <- struct{}{}:
	default:
		return Run{}, ErrCapacity
	}
	var source Source
	run := Run{ID: auth.Token(), SourceID: id, Status: "running", StartedAt: time.Now().UTC(), RequestID: requestID}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND enabled", id).Take(&source).Error; err != nil {
			return err
		}
		if source.Provider != "netease" {
			return provider.InvalidSource
		}
		if source.SyncStatus == "running" {
			return ErrBusy
		}
		if !since.IsZero() {
			var count int64
			if err := tx.Model(&Run{}).Where("source_id=? AND started_at>=?", id, since).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return errAttempted
			}
		}
		if err := tx.Create(&run).Error; err != nil {
			return err
		}
		return tx.Model(&Source{}).Where("id=?", id).Updates(map[string]any{"sync_status": "running", "updated_at": time.Now().UTC()}).Error
	})
	if err != nil {
		<-s.slots
		return Run{}, err
	}
	s.workers.Add(1)
	go func() { defer s.workers.Done(); defer func() { <-s.slots }(); s.execute(source, run) }()
	return run, nil
}

// SyncDaily attempts each source at most once per business date, before recommendations.
func (s *Service) SyncDaily(ctx context.Context, now time.Time) error {
	sources, err := s.Playlists(ctx)
	if err != nil {
		return err
	}
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	for _, source := range sources {
		if source.Provider != "netease" {
			continue
		}
		run, err := s.startSync(ctx, source.ID, "scheduler", midnight)
		if errors.Is(err, errAttempted) || errors.Is(err, ErrBusy) {
			continue
		}
		if err != nil {
			return err
		}
		ticker := time.NewTicker(200 * time.Millisecond)
		for run.Status == "running" {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return ctx.Err()
			case <-ticker.C:
			}
			run, err = s.GetRun(ctx, run.ID)
			if err != nil {
				ticker.Stop()
				return err
			}
		}
		ticker.Stop()
	}
	return nil
}
func (s *Service) execute(source Source, run Run) {
	ctx, cancel := context.WithTimeout(s.Context, 30*time.Second)
	defer cancel()
	items, err := s.Adapter.FetchPlaylist(ctx, provider.Ref{Provider: source.Provider, ExternalID: source.ExternalID, Title: source.Title, SourceURL: source.SourceURL, EmbedURL: source.EmbedURL})
	if err == nil {
		err = provider.Validate(items, source.Provider)
	}
	if err == nil {
		err = s.ReplaceSnapshot(ctx, source, run, items)
	}
	if err != nil {
		cleanup, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		s.fail(cleanup, run, provider.Code(err))
	}
}
func (s *Service) fail(ctx context.Context, run Run, code string) error {
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var source Source
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", run.SourceID).Take(&source).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		result := tx.Model(&Run{}).Where("id=? AND status='running'", run.ID).Updates(map[string]any{"status": "failed", "finished_at": now, "error_code": code})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		return tx.Model(&Source{}).Where("id=?", run.SourceID).Updates(map[string]any{"sync_status": "failed", "last_error_code": code, "last_error_at": now, "updated_at": now}).Error
	})
}
func (s *Service) Recover(ctx context.Context) error {
	// Only expired leases are reclaimed, so another live instance is not interrupted.
	var runs []Run
	if err := s.DB.WithContext(ctx).Where("status='running' AND started_at < ?", time.Now().UTC().Add(-2*time.Minute)).Find(&runs).Error; err != nil {
		return err
	}
	for _, run := range runs {
		if err := s.fail(ctx, run, "SYNC_INTERRUPTED"); err != nil {
			return err
		}
	}
	return nil
}
func (s *Service) ReplaceSnapshot(ctx context.Context, source Source, run Run, items []provider.Track) error {
	if err := provider.Validate(items, source.Provider); err != nil {
		return err
	}
	encoded, _ := json.Marshal(items)
	sum := sha256.Sum256(encoded)
	hash := hex.EncodeToString(sum[:])
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current Source
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", source.ID).Take(&current).Error; err != nil {
			return err
		}
		var active Run
		if err := tx.Where("id=? AND status='running'", run.ID).Take(&active).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		if current.SnapshotHash != hash {
			if err := tx.Exec("UPDATE source_items SET active=false WHERE source_id=?", source.ID).Error; err != nil {
				return err
			}
			for i, t := range items {
				part := ""
				if t.PartID != nil {
					part = *t.PartID
				}
				if err := tx.Exec(`INSERT INTO music_items(id,provider,external_id,part_key,title,author,source_url,duration_seconds,availability,embed_url) VALUES (?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET title=EXCLUDED.title,author=EXCLUDED.author,source_url=EXCLUDED.source_url,duration_seconds=EXCLUDED.duration_seconds,availability=EXCLUDED.availability,embed_url=EXCLUDED.embed_url,updated_at=now()`, t.ID, t.Provider, t.ExternalID, part, t.Title, t.Author, t.SourceURL, t.DurationSeconds, t.Availability, t.EmbedURL).Error; err != nil {
					return err
				}
				if err := tx.Exec(`INSERT INTO source_items(source_id,item_id,position) VALUES (?,?,?) ON CONFLICT(source_id,item_id) DO UPDATE SET position=EXCLUDED.position,active=true,last_seen=now()`, source.ID, t.ID, i).Error; err != nil {
					return err
				}
			}
		}
		if err := tx.Model(&Source{}).Where("id=?", source.ID).Updates(map[string]any{"sync_status": "ready", "synced_at": now, "snapshot_hash": hash, "last_error_code": nil, "last_error_at": nil, "updated_at": now}).Error; err != nil {
			return err
		}
		return tx.Model(&Run{}).Where("id=?", run.ID).Updates(map[string]any{"status": "ready", "finished_at": now, "candidate_count": len(items)}).Error
	})
}
