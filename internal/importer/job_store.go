package importer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// JobRecord is the stored form of a user's latest import job. While the job
// runs it also holds the entries still to import, so the job can be resumed
// after a restart.
type JobRecord struct {
	UserID    string   `gorm:"primaryKey;column:user_id"`
	State     JobState `gorm:"column:state;not null"`
	Total     int      `gorm:"column:total;not null"`
	Processed int      `gorm:"column:processed;not null"`
	Source    string   `gorm:"column:source"`
	Enrich    bool     `gorm:"column:enrich"`
	// Entries is the JSON of the uploaded rows; cleared once the job is done.
	Entries string `gorm:"column:entries;type:text"`
	// Result is the JSON of the outcome so far.
	Result    string `gorm:"column:result;type:text"`
	UpdatedAt time.Time
}

// TableName overrides the GORM table name.
func (*JobRecord) TableName() string {
	return "import_jobs"
}

// JobStore keeps import jobs in the database.
type JobStore struct {
	db *gorm.DB
}

// NewJobStore returns a JobRepository backed by db.
func NewJobStore(db *gorm.DB) *JobStore {
	return &JobStore{db: db}
}

// Start registers a new running job for the user.
func (s *JobStore) Start(ctx context.Context, job *PendingJob) error {
	entries, err := json.Marshal(job.Entries)
	if err != nil {
		return fmt.Errorf("encode entries: %w", err)
	}

	record := JobRecord{
		UserID: job.UserID, State: JobRunning, Total: len(job.Entries),
		Source: job.Source, Enrich: job.Enrich, Entries: string(entries),
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var running int64

		err := tx.Model(&JobRecord{}).Where("user_id = ? AND state = ?", job.UserID, JobRunning).Count(&running).Error
		if err != nil {
			return err
		}

		if running > 0 {
			return ErrImportRunning
		}

		return tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&record).Error
	})
	if err != nil {
		return fmt.Errorf("store import job: %w", err)
	}

	return nil
}

// Progress records how far a running job has got.
func (s *JobStore) Progress(ctx context.Context, userID string, processed int, result *ImportResult) error {
	return s.update(ctx, userID, map[string]any{"processed": processed}, result)
}

// Finish marks a job as completed and drops the stored entries.
func (s *JobStore) Finish(ctx context.Context, userID string, result *ImportResult) error {
	return s.update(ctx, userID, map[string]any{
		"state":     JobCompleted,
		"processed": gorm.Expr("total"),
		"entries":   "",
	}, result)
}

// Get returns the user's latest job.
func (s *JobStore) Get(ctx context.Context, userID string) (Job, bool) {
	var record JobRecord

	err := s.db.WithContext(ctx).Omit("entries").First(&record, "user_id = ?", userID).Error
	if err != nil {
		return Job{}, false
	}

	job := Job{State: record.State, Total: record.Total, Processed: record.Processed}

	// The result is only shown once the job is done.
	if record.State == JobCompleted {
		job.Result = decodeResult(record.Result)
	}

	return job, true
}

// Interrupted returns the jobs that were running when the process stopped.
func (s *JobStore) Interrupted(ctx context.Context) ([]*PendingJob, error) {
	var records []JobRecord

	err := s.db.WithContext(ctx).Where("state = ?", JobRunning).Find(&records).Error
	if err != nil {
		return nil, fmt.Errorf("find running import jobs: %w", err)
	}

	jobs := make([]*PendingJob, 0, len(records))

	for i := range records {
		job, err := records[i].pending()
		if err != nil {
			// Unreadable input cannot be resumed; close the job instead.
			_ = s.Finish(ctx, records[i].UserID, &ImportResult{Total: records[i].Total, Failed: records[i].Total})

			continue
		}

		jobs = append(jobs, job)
	}

	return jobs, nil
}

func (s *JobStore) update(ctx context.Context, userID string, fields map[string]any, result *ImportResult) error {
	encoded, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("encode result: %w", err)
	}

	fields["result"] = string(encoded)
	fields["updated_at"] = time.Now()

	err = s.db.WithContext(ctx).Model(&JobRecord{}).Where("user_id = ?", userID).Updates(fields).Error
	if err != nil {
		return fmt.Errorf("update import job: %w", err)
	}

	return nil
}

var errNoEntries = errors.New("job has no stored entries")

func (r *JobRecord) pending() (*PendingJob, error) {
	var entries []ImportEntry

	err := json.Unmarshal([]byte(r.Entries), &entries)
	if err != nil {
		return nil, fmt.Errorf("decode entries: %w", err)
	}

	if len(entries) == 0 {
		return nil, errNoEntries
	}

	return &PendingJob{
		UserID: r.UserID, Source: r.Source, Enrich: r.Enrich, Entries: entries,
		Processed: min(r.Processed, len(entries)), Result: decodeResult(r.Result),
	}, nil
}

func decodeResult(encoded string) *ImportResult {
	var result ImportResult

	err := json.Unmarshal([]byte(encoded), &result)
	if err != nil {
		return nil
	}

	return &result
}
