package importer

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func newTestJobStore(t *testing.T) *JobStore {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	err = db.AutoMigrate(&JobRecord{})
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	return NewJobStore(db)
}

func fourEntries() []ImportEntry {
	return []ImportEntry{importEntry("KL1001"), importEntry("KL1003"), importEntry("KL1005"), importEntry("KL1007")}
}

// interruptJob leaves behind what a stopped server would: a running job with
// two of its four entries done.
func interruptJob(t *testing.T, store *JobStore, userID string) {
	t.Helper()

	err := store.Start(t.Context(), &PendingJob{UserID: userID, Source: "flightlog", Entries: fourEntries()})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	err = store.Progress(t.Context(), userID, 2, &ImportResult{Total: 4, Imported: 1, Skipped: 1})
	if err != nil {
		t.Fatalf("Progress: %v", err)
	}
}

// TestImportResumesAfterRestart simulates a server that stopped part-way
// through an import: a new service on the same database must finish the job
// from where it stopped, without importing the first entries twice.
func TestImportResumesAfterRestart(t *testing.T) {
	t.Parallel()

	store := newTestJobStore(t)
	interruptJob(t, store, "user-1")

	// The restarted server.
	linker := &fakeLinker{}
	svc := NewService(nil, linker, &fakeCache{}, nil).WithJobRepository(store)

	if resumed := svc.ResumeInterrupted(context.Background()); resumed != 1 {
		t.Fatalf("ResumeInterrupted = %d, want 1", resumed)
	}

	job := waitForJob(t, svc, "user-1", func(j Job) bool { return j.State == JobCompleted })

	if job.Result == nil {
		t.Fatalf("finished job = %+v, want a result", job)
	}

	// One import and one skip were counted before the restart.
	want := ImportResult{Total: 4, Imported: 3, Skipped: 1}
	if got := *job.Result; got.Total != want.Total || got.Imported != want.Imported || got.Skipped != want.Skipped {
		t.Errorf("result = %+v, want %+v", got, want)
	}

	if len(linker.linked) != 2 {
		t.Errorf("flights linked after resuming = %d, want only the 2 that were left", len(linker.linked))
	}
}

func TestInterruptedImportBlocksANewOneUntilResumed(t *testing.T) {
	t.Parallel()

	store := newTestJobStore(t)
	interruptJob(t, store, "user-2")

	ctx := context.Background()
	svc := NewService(nil, &fakeLinker{}, &fakeCache{}, nil).WithJobRepository(store)

	_, err := svc.StartImport(ctx, "user-2", fourEntries(), "flightlog", false)
	if !errors.Is(err, ErrImportRunning) {
		t.Errorf("StartImport while a job is interrupted: err = %v, want ErrImportRunning", err)
	}

	svc.ResumeInterrupted(ctx)
	waitForJob(t, svc, "user-2", func(j Job) bool { return j.State == JobCompleted })

	if resumed := svc.ResumeInterrupted(ctx); resumed != 0 {
		t.Errorf("ResumeInterrupted after completion = %d, want 0", resumed)
	}

	_, err = svc.StartImport(ctx, "user-2", fourEntries()[:1], "flightlog", false)
	if err != nil {
		t.Errorf("StartImport after completion: %v", err)
	}

	// Let the background job finish before the test's database is removed.
	waitForJob(t, svc, "user-2", func(j Job) bool { return j.State == JobCompleted && j.Total == 1 })
}

func TestJobStoreKeepsEntryDetailsForResume(t *testing.T) {
	t.Parallel()

	store := newTestJobStore(t)
	ctx := context.Background()
	entry := importEntry("KL1001")
	entry.DepLat, entry.DepGate, entry.AirlineICAO = 52.31, "D7", "KLM"

	err := store.Start(ctx, &PendingJob{UserID: "user-1", Source: "flighty", Enrich: true, Entries: []ImportEntry{entry}})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// A running job reports progress but no result yet.
	job, _ := store.Get(ctx, "user-1")
	if want := (Job{State: JobRunning, Total: 1}); job != want {
		t.Errorf("stored job = %+v, want %+v", job, want)
	}

	pending, err := store.Interrupted(ctx)
	if err != nil || len(pending) != 1 {
		t.Fatalf("Interrupted = %d jobs, err %v; want 1", len(pending), err)
	}

	got := pending[0]
	if got.Source != "flighty" || !got.Enrich {
		t.Errorf("resumed job = %+v, want the original source and enrich flag", got)
	}

	if len(got.Entries) != 1 || got.Entries[0] != entry {
		t.Errorf("resumed entries = %+v, want the original entry", got.Entries)
	}
}
