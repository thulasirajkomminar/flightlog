package importer

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"go.uber.org/zap"
)

// ErrImportRunning is returned when a user starts an import while their
// previous one is still in progress.
var ErrImportRunning = errors.New("an import is already running")

var errJobPanic = errors.New("panic")

// JobState is the lifecycle state of a background import.
type JobState string

// Background import states.
const (
	JobRunning   JobState = "running"
	JobCompleted JobState = "completed"
)

// Job is a snapshot of a user's background import.
type Job struct {
	State     JobState      `json:"state"`
	Total     int           `json:"total"`
	Processed int           `json:"processed"`
	Result    *ImportResult `json:"result,omitempty"`
}

// PendingJob is everything needed to run, or carry on with, an import.
type PendingJob struct {
	UserID  string
	Source  string
	Enrich  bool
	Entries []ImportEntry
	// Processed entries are skipped when the job is resumed, and Result holds
	// the outcome counted so far.
	Processed int
	Result    *ImportResult
}

// JobRepository keeps the latest import job of each user.
type JobRepository interface {
	// Start registers a new running job, replacing a finished one. It returns
	// ErrImportRunning while the user's previous job is still running.
	Start(ctx context.Context, job *PendingJob) error
	// Progress records how far a running job has got.
	Progress(ctx context.Context, userID string, processed int, result *ImportResult) error
	// Finish marks a job as completed with its final result.
	Finish(ctx context.Context, userID string, result *ImportResult) error
	// Get returns the user's latest job.
	Get(ctx context.Context, userID string) (Job, bool)
	// Interrupted returns jobs that were still running when the process last
	// stopped, so they can be resumed.
	Interrupted(ctx context.Context) ([]*PendingJob, error)
}

// memoryJobs keeps jobs in memory. Jobs do not survive a restart; it is the
// default until WithJobRepository supplies a durable store.
type memoryJobs struct {
	mu   sync.Mutex
	jobs map[string]*Job
}

func newMemoryJobs() *memoryJobs {
	return &memoryJobs{jobs: make(map[string]*Job)}
}

func (m *memoryJobs) Start(_ context.Context, job *PendingJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, ok := m.jobs[job.UserID]; ok && existing.State == JobRunning {
		return ErrImportRunning
	}

	m.jobs[job.UserID] = &Job{State: JobRunning, Total: len(job.Entries)}

	return nil
}

func (m *memoryJobs) Progress(_ context.Context, userID string, processed int, _ *ImportResult) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if job, ok := m.jobs[userID]; ok {
		job.Processed = processed
	}

	return nil
}

func (m *memoryJobs) Finish(_ context.Context, userID string, result *ImportResult) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if job, ok := m.jobs[userID]; ok {
		job.State = JobCompleted
		job.Processed = job.Total
		job.Result = result
	}

	return nil
}

func (m *memoryJobs) Get(_ context.Context, userID string) (Job, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.jobs[userID]
	if !ok {
		return Job{}, false
	}

	return *job, true
}

func (*memoryJobs) Interrupted(context.Context) ([]*PendingJob, error) {
	return nil, nil
}

// WithJobRepository makes import jobs durable, so a running import survives a
// restart (see ResumeInterrupted).
func (s *Service) WithJobRepository(jobs JobRepository) *Service {
	s.jobs = jobs

	return s
}

// StartImport begins importing entries in the background and returns the new
// job. Progress is available through ImportStatus. ctx must outlive the
// request that started the import.
func (s *Service) StartImport(ctx context.Context, userID string, entries []ImportEntry, source string, enrich bool) (Job, error) {
	pending := &PendingJob{UserID: userID, Source: source, Enrich: enrich, Entries: entries}

	err := s.jobs.Start(ctx, pending)
	if err != nil {
		return Job{}, fmt.Errorf("start import job: %w", err)
	}

	go s.runJob(ctx, pending)

	return Job{State: JobRunning, Total: len(entries)}, nil
}

// ImportStatus returns the user's running or most recently finished import.
func (s *Service) ImportStatus(ctx context.Context, userID string) (Job, bool) {
	return s.jobs.Get(ctx, userID)
}

// ResumeInterrupted carries on with imports that were still running when the
// server last stopped, from the entry they had reached. It returns how many
// jobs were resumed.
func (s *Service) ResumeInterrupted(ctx context.Context) int {
	pending, err := s.jobs.Interrupted(ctx)
	if err != nil {
		s.log.Error("failed to load interrupted imports", err)

		return 0
	}

	for _, job := range pending {
		s.log.Info("resuming interrupted import",
			zap.String(logKeyUserID, job.UserID), zap.Int("processed", job.Processed), zap.Int("total", len(job.Entries)))

		go s.runJob(ctx, job)
	}

	return len(pending)
}

func (s *Service) runJob(ctx context.Context, job *PendingJob) {
	// A panic here would take the whole server down; fail the job instead.
	defer func() {
		if r := recover(); r != nil {
			s.log.Error("import job panicked", fmt.Errorf("%w: %v", errJobPanic, r), zap.String(logKeyUserID, job.UserID))
			s.finishJob(ctx, job.UserID, &ImportResult{Total: len(job.Entries), Failed: len(job.Entries)})
		}
	}()

	result := s.run(ctx, job, func(processed int, soFar *ImportResult) {
		err := s.jobs.Progress(ctx, job.UserID, processed, soFar)
		if err != nil {
			s.log.Warn("failed to record import progress", zap.Error(err))
		}
	})

	s.finishJob(ctx, job.UserID, result)
}

func (s *Service) finishJob(ctx context.Context, userID string, result *ImportResult) {
	err := s.jobs.Finish(ctx, userID, result)
	if err != nil {
		s.log.Error("failed to record finished import", err, zap.String(logKeyUserID, userID))
	}
}
