//go:build unit

package service

import (
	"testing"
	"time"
)

func newTestHealthTracker(now *time.Time) *AccountHealthTracker {
	t := NewAccountHealthTracker()
	t.now = func() time.Time { return *now }
	return t
}

func TestAccountHealthTracker_NoDataScoresEqual(t *testing.T) {
	now := time.Now()
	tr := newTestHealthTracker(&now)
	scores := tr.Scores([]int64{1, 2, 3})
	if !healthScoresAllEqual(scores) {
		t.Fatalf("expected equal scores without data, got %v", scores)
	}
	for _, s := range scores {
		if s != 1 {
			t.Fatalf("expected neutral score 1, got %v", s)
		}
	}
}

func TestAccountHealthTracker_ErrorsLowerScoreAndDecay(t *testing.T) {
	now := time.Now()
	tr := newTestHealthTracker(&now)
	for i := 0; i < 10; i++ {
		tr.Report(1, false, nil)
		tr.Report(2, true, nil)
	}
	scores := tr.Scores([]int64{1, 2})
	if scores[0] >= 0.2 || scores[1] != 1 {
		t.Fatalf("unexpected scores %v", scores)
	}
	now = now.Add(60 * time.Minute) // 6 个半衰期
	scores = tr.Scores([]int64{1, 2})
	if scores[0] < 0.95 {
		t.Fatalf("expected error rate to decay, got %v", scores[0])
	}
}

func TestAccountHealthTracker_SlowTTFTPenalized(t *testing.T) {
	now := time.Now()
	tr := newTestHealthTracker(&now)
	fast, slow := 800, 8000
	for i := 0; i < 5; i++ {
		tr.Report(1, true, &fast)
		tr.Report(2, true, &fast)
		tr.Report(3, true, &slow)
	}
	scores := tr.Scores([]int64{1, 2, 3})
	if scores[0] != 1 || scores[1] != 1 {
		t.Fatalf("fast accounts should keep full score: %v", scores)
	}
	if scores[2] > 0.5 || scores[2] < healthTTFTMinFactor {
		t.Fatalf("slow account score out of range: %v", scores[2])
	}
}

func TestAccountHealthTracker_PenaltyAndExpiry(t *testing.T) {
	now := time.Now()
	tr := newTestHealthTracker(&now)
	tr.SetPenalty(1, 0.2, now.Add(time.Hour), "iq_failed")
	snap := tr.Snapshot(1)
	if snap.PenaltyFactor != 0.2 || snap.Score != 0.2 || snap.PenaltyReason != "iq_failed" {
		t.Fatalf("unexpected snapshot %+v", snap)
	}
	now = now.Add(2 * time.Hour)
	if s := tr.Snapshot(1); s.PenaltyFactor != 1 || s.Score != 1 {
		t.Fatalf("penalty should expire: %+v", s)
	}
	tr.SetPenalty(2, 0.5, time.Time{}, "manual")
	tr.ClearPenalty(2)
	if s := tr.Snapshot(2); s.PenaltyFactor != 1 {
		t.Fatalf("penalty should be cleared: %+v", s)
	}
}

func TestAccountHealthTracker_PickWeightedDistribution(t *testing.T) {
	now := time.Now()
	tr := newTestHealthTracker(&now)
	for i := 0; i < 20; i++ {
		tr.Report(2, false, nil)
	}
	counts := map[int]int{}
	ids := []int64{1, 2}
	for i := 0; i < 10000; i++ {
		counts[tr.PickWeighted(ids, []int{0, 0})]++
	}
	if counts[1] == 0 {
		t.Fatalf("bad account should still get probe traffic")
	}
	if counts[1] > 800 {
		t.Fatalf("bad account got too much traffic: %v", counts)
	}
}

func TestReorderSegmentByHealth_EqualKeepsOrder(t *testing.T) {
	now := time.Now()
	tr := newTestHealthTracker(&now)
	seg := []accountWithLoad{
		{account: &Account{ID: 1}, loadInfo: &AccountLoadInfo{}},
		{account: &Account{ID: 2}, loadInfo: &AccountLoadInfo{}},
		{account: &Account{ID: 3}, loadInfo: &AccountLoadInfo{}},
	}
	reorderSegmentByHealth(tr, seg)
	for i, want := range []int64{1, 2, 3} {
		if seg[i].account.ID != want {
			t.Fatalf("order changed without health difference")
		}
	}
}

func TestReorderSegmentByHealth_BadAccountMovesBack(t *testing.T) {
	now := time.Now()
	tr := newTestHealthTracker(&now)
	for i := 0; i < 20; i++ {
		tr.Report(1, false, nil)
	}
	firstBad := 0
	for n := 0; n < 2000; n++ {
		seg := []accountWithLoad{
			{account: &Account{ID: 1}, loadInfo: &AccountLoadInfo{}},
			{account: &Account{ID: 2}, loadInfo: &AccountLoadInfo{}},
			{account: &Account{ID: 3}, loadInfo: &AccountLoadInfo{}},
		}
		reorderSegmentByHealth(tr, seg)
		if seg[0].account.ID == 1 {
			firstBad++
		}
	}
	if firstBad > 100 {
		t.Fatalf("bad account placed first too often: %d/2000", firstBad)
	}
}
