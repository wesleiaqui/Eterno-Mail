package sync

import "testing"

// TestShouldUseCondStore walks every branch of the truth table the docs
// describe. Each "no" branch tested in isolation so a future refactor that
// drops one of them surfaces immediately.
func TestShouldUseCondStore(t *testing.T) {
	cases := []struct {
		name               string
		uidValidityChanged bool
		prevModSeq         uint64
		mailboxModSeq      uint64
		supportsCondStore  bool
		want               bool
	}{
		{
			name:               "all good: use CONDSTORE",
			uidValidityChanged: false,
			prevModSeq:         100,
			mailboxModSeq:      200,
			supportsCondStore:  true,
			want:               true,
		},
		{
			name:               "UIDValidity changed: must full-sync",
			uidValidityChanged: true,
			prevModSeq:         100,
			mailboxModSeq:      200,
			supportsCondStore:  true,
			want:               false,
		},
		{
			name:               "prevModSeq=0 (first sync ever): must full-sync to capture baseline",
			uidValidityChanged: false,
			prevModSeq:         0,
			mailboxModSeq:      200,
			supportsCondStore:  true,
			want:               false,
		},
		{
			name:               "server didn't return HIGHESTMODSEQ this round: full-sync, don't trust the path",
			uidValidityChanged: false,
			prevModSeq:         100,
			mailboxModSeq:      0,
			supportsCondStore:  true,
			want:               false,
		},
		{
			name:               "server lacks CONDSTORE capability: always full",
			uidValidityChanged: false,
			prevModSeq:         100,
			mailboxModSeq:      200,
			supportsCondStore:  false,
			want:               false,
		},
		{
			name:               "all four no-conditions at once: still false",
			uidValidityChanged: true,
			prevModSeq:         0,
			mailboxModSeq:      0,
			supportsCondStore:  false,
			want:               false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldUseCondStore(tc.uidValidityChanged, tc.prevModSeq, tc.mailboxModSeq, tc.supportsCondStore)
			if got != tc.want {
				t.Errorf("shouldUseCondStore(uidValidityChanged=%v, prevModSeq=%d, mailboxModSeq=%d, supportsCondStore=%v) = %v, want %v",
					tc.uidValidityChanged, tc.prevModSeq, tc.mailboxModSeq, tc.supportsCondStore, got, tc.want)
			}
		})
	}
}

func TestCondStoreFallbackReason(t *testing.T) {
	tests := []struct {
		name               string
		uidValidityChanged bool
		prevModSeq         uint64
		mailboxModSeq      uint64
		supports           bool
		preferIncremental  bool
		existing           int
		periodicSweep      bool
		want               string
	}{
		{name: "first sync", mailboxModSeq: 10, supports: true, preferIncremental: true, want: "no_stored_modseq"},
		{name: "idle second sync", prevModSeq: 10, mailboxModSeq: 11, supports: true, preferIncremental: true, want: ""},
		{name: "scheduled small mailbox", prevModSeq: 10, mailboxModSeq: 11, supports: true, existing: 39, want: "below_full_reconcile_threshold"},
		{name: "uidvalidity changed", uidValidityChanged: true, prevModSeq: 10, mailboxModSeq: 11, supports: true, preferIncremental: true, want: "uidvalidity_changed"},
		{name: "server lacks condstore", prevModSeq: 10, mailboxModSeq: 11, preferIncremental: true, want: "condstore_unavailable"},
		{name: "server omitted mailbox modseq", prevModSeq: 10, supports: true, preferIncremental: true, want: "no_mailbox_modseq"},
		{name: "periodic full sweep", prevModSeq: 10, mailboxModSeq: 11, supports: true, existing: flagFullReconcileThreshold, periodicSweep: true, want: "periodic_full_sweep"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := condStoreFallbackReason(test.uidValidityChanged, test.prevModSeq, test.mailboxModSeq, test.supports, test.preferIncremental, test.existing, test.periodicSweep)
			if got != test.want {
				t.Fatalf("condStoreFallbackReason() = %q, want %q", got, test.want)
			}
		})
	}
}

// TestNextModSeq_FlagSyncOK_AdvancesToMailbox: the happy path. Sync succeeded,
// server reported a fresh HIGHESTMODSEQ — persist that, the next cycle will
// CHANGEDSINCE from there.
func TestNextModSeq_FlagSyncOK_AdvancesToMailbox(t *testing.T) {
	got := nextModSeq(true /*flagSyncOK*/, 500 /*mailboxModSeq*/, 100 /*prevModSeq*/)
	if got != 500 {
		t.Errorf("nextModSeq(ok=true, mailbox=500, prev=100) = %d, want 500", got)
	}
}

// TestNextModSeq_FlagSyncFailed_PinsToPrev: THE safety invariant of this PR.
// If a flag sync didn't succeed and we still advance the baseline, the next
// cycle's CHANGEDSINCE filter silently skips whatever the failed cycle
// missed. Forever. nextModSeq exists specifically to prevent that — and the
// test exists specifically to make it impossible to break by mistake.
func TestNextModSeq_FlagSyncFailed_PinsToPrev(t *testing.T) {
	got := nextModSeq(false /*flagSyncOK*/, 500 /*mailboxModSeq*/, 100 /*prevModSeq*/)
	if got != 100 {
		t.Errorf("nextModSeq(ok=false, mailbox=500, prev=100) = %d, want 100 (must pin on failure)", got)
	}
}

// TestNextModSeq_FlagSyncOK_ButMailboxZero: even on a successful flag sync,
// if the server didn't report HIGHESTMODSEQ this round we can't advance —
// advancing to 0 would degenerate the next CONDSTORE check (sinceModSeq=0
// returns the whole mailbox).
func TestNextModSeq_FlagSyncOK_ButMailboxZero(t *testing.T) {
	got := nextModSeq(true /*flagSyncOK*/, 0 /*mailboxModSeq*/, 100 /*prevModSeq*/)
	if got != 100 {
		t.Errorf("nextModSeq(ok=true, mailbox=0, prev=100) = %d, want 100 (must pin when mailbox modseq is 0)", got)
	}
}

// TestNextModSeq_PrevZero_AdvancesOnSuccess: first-ever sync. prev was 0,
// flag sync ran (the full path, since shouldUseCondStore returned false),
// it succeeded, server reported a modseq. We DO want to advance from 0 →
// mailboxModSeq so the next cycle can use the incremental path.
func TestNextModSeq_PrevZero_AdvancesOnSuccess(t *testing.T) {
	got := nextModSeq(true /*flagSyncOK*/, 500 /*mailboxModSeq*/, 0 /*prevModSeq*/)
	if got != 500 {
		t.Errorf("nextModSeq(ok=true, mailbox=500, prev=0) = %d, want 500 (first-sync advancement)", got)
	}
}

// TestDueForFullFlagSweep verifies the periodic full-sweep cadence that lets a
// broken/partial CONDSTORE self-heal: every flagFullSweepEvery-th call returns
// true, per folder, independently.
func TestDueForFullFlagSweep(t *testing.T) {
	e := &Engine{flagSweepCounter: map[string]int{}}

	fulls := 0
	for i := 1; i <= flagFullSweepEvery*2; i++ {
		if e.dueForFullFlagSweep("inbox") {
			fulls++
			if i%flagFullSweepEvery != 0 {
				t.Errorf("full sweep fired at cycle %d, expected only multiples of %d", i, flagFullSweepEvery)
			}
		}
	}
	if fulls != 2 {
		t.Errorf("expected 2 full sweeps over %d cycles, got %d", flagFullSweepEvery*2, fulls)
	}

	// Counters are independent per folder.
	if e.dueForFullFlagSweep("other") {
		t.Error("first call for a new folder should not trigger a full sweep")
	}
}
