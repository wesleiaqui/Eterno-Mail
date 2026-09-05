package app

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func testAccountJobs(count int) []accountSyncJob {
	jobs := make([]accountSyncJob, count)
	for i := range jobs {
		jobs[i] = accountSyncJob{accountID: string(rune('a' + i))}
	}
	return jobs
}

func TestRunAccountSyncsSingleAccount(t *testing.T) {
	called := 0
	results, maxObserved := runAccountSyncs(
		testAccountJobs(1), 2, func() bool { return false },
		func(string) error { called++; return nil }, nil, nil,
	)
	if called != 1 || maxObserved != 1 || !results[0].started {
		t.Fatalf("called=%d max=%d results=%#v, want one started worker", called, maxObserved, results)
	}
}

func TestRunAccountSyncsHonorsConcurrencyLimitAndProcessesQueue(t *testing.T) {
	started := make(chan string, 5)
	release := make(chan struct{})
	done := make(chan struct{})
	var mu sync.Mutex
	called := 0
	var results []accountSyncResult
	maxObserved := 0

	go func() {
		results, maxObserved = runAccountSyncs(
			testAccountJobs(5), 2, func() bool { return false },
			func(id string) error {
				mu.Lock()
				called++
				mu.Unlock()
				started <- id
				<-release
				return nil
			}, nil, nil,
		)
		close(done)
	}()

	<-started
	<-started
	select {
	case <-done:
		t.Fatal("orchestrator returned before active workers were released")
	default:
	}
	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("orchestrator did not finish queued jobs")
	}

	mu.Lock()
	defer mu.Unlock()
	if called != 5 || maxObserved != 2 {
		t.Fatalf("called=%d max=%d, want all 5 jobs with max 2", called, maxObserved)
	}
	for i, result := range results {
		if !result.started || result.err != nil {
			t.Fatalf("result %d = %#v, want successful started job", i, result)
		}
	}
}

func TestRunAccountSyncsContinuesAndKeepsResultsInInputOrder(t *testing.T) {
	errA := errors.New("A failed")
	errC := errors.New("C failed")
	releaseA := make(chan struct{})
	startedA := make(chan struct{})
	finishedC := make(chan struct{})
	done := make(chan struct{})
	var results []accountSyncResult

	go func() {
		results, _ = runAccountSyncs(
			testAccountJobs(3), 2, func() bool { return false },
			func(id string) error {
				switch id {
				case "a":
					close(startedA)
					<-releaseA
					return errA
				case "c":
					close(finishedC)
					return errC
				default:
					return nil
				}
			}, nil, nil,
		)
		close(done)
	}()

	<-startedA
	<-finishedC // C completes before A, exercising deterministic result order.
	close(releaseA)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("orchestrator did not finish after independent error")
	}
	if !results[0].started || !results[1].started || !results[2].started {
		t.Fatalf("results=%#v, want every account to run", results)
	}
	if !errors.Is(results[0].err, errA) || results[1].err != nil || !errors.Is(results[2].err, errC) {
		t.Fatalf("results=%#v, want errors in input positions A then C", results)
	}
}

func TestRunAccountSyncsCancellationLeavesQueuedAccountsUnstarted(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	var cancelMu sync.Mutex
	cancelled := false
	var results []accountSyncResult

	go func() {
		results, _ = runAccountSyncs(
			testAccountJobs(3), 1,
			func() bool {
				cancelMu.Lock()
				defer cancelMu.Unlock()
				return cancelled
			},
			func(string) error {
				started <- struct{}{}
				<-release
				return nil
			}, nil, nil,
		)
		close(done)
	}()

	<-started
	cancelMu.Lock()
	cancelled = true
	cancelMu.Unlock()
	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("orchestrator did not wait for the started worker")
	}
	if !results[0].started || results[1].started || results[2].started {
		t.Fatalf("results=%#v, want only the already-started account", results)
	}
}
