package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

const checkinTestHash = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestCheckinSpendThresholds(t *testing.T) {
	for _, tc := range []struct{ amount, want float64 }{{-1, 0}, {0, 0}, {0.5, 1.5}, {1, 3}, {3, 5.5}, {5, 8}, {7.5, 10}, {10, 12}, {15, 13.5}, {20, 15}, {1000, 15}} {
		require.InDelta(t, tc.want, checkinSpendScore(tc.amount), 1e-9)
	}
}

func TestCheckinProbabilityEndpointsAndEveryTicket(t *testing.T) {
	require.Equal(t, []CheckinPrize{{1, 65}, {3, 25}, {10, 8}, {50, 1.5}, {100, 0.5}}, checkinPrizes(0))
	require.Equal(t, []CheckinPrize{{1, 25}, {3, 30}, {10, 30}, {50, 12}, {100, 3}}, checkinPrizes(100))
	require.InDelta(t, 1.75, checkinPrizes(50)[4].Probability, 1e-9)
	for _, score := range []float64{0, 5.7142857, 50, 68.142857, 100} {
		prizes := checkinPrizes(score)
		counts := map[int]int64{}
		// Exhaust the sample space: tests distribution and every boundary without flaky randomness.
		for ticket := int64(0); ticket < 1000000; ticket++ {
			counts[checkinReward(prizes, ticket)]++
		}
		for _, p := range prizes {
			require.Equal(t, int64(math.Round(p.Probability*10000)), counts[p.Cents])
		}
		require.LessOrEqual(t, prizes[4].Probability, 3.0)
	}
}

func TestCheckinScoreCapsAndStreakOnly(t *testing.T) {
	full := calculateCheckinSnapshot(100, 100, 999, true)
	require.Equal(t, 100.0, full.Score)
	require.Equal(t, 7, full.ActiveDays)
	require.Equal(t, 7, full.StreakDays)
	streak := calculateCheckinSnapshot(0, 7, 0, false)
	require.Equal(t, 40.0, streak.Score)
	require.Equal(t, 1.5, streak.Prizes[4].Probability)
	// First claim includes today, giving 40/7 streak points.
	require.InDelta(t, 40.0/7, calculateCheckinSnapshot(0, 1, 0, false).Score, 1e-9)
}

func TestCheckinBeijingMidnight(t *testing.T) {
	before := time.Date(2026, 9, 18, 15, 59, 59, 0, time.UTC)
	require.Equal(t, "2026-09-18", checkinDay(before).Format(time.DateOnly))
	require.Equal(t, "2026-09-19", checkinDay(before.Add(time.Second)).Format(time.DateOnly))
}

func newCheckinTestService(t *testing.T) (*CheckinService, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	s := NewCheckinService(db, nil, nil, nil, nil)
	s.settings = checkinSwitchStub(true)
	s.now = func() time.Time { return time.Date(2026, 9, 18, 10, 0, 0, 0, checkinZone) }
	s.draw = func(n int64) (int64, error) { require.Equal(t, int64(1000000), n); return 0, nil }
	return s, mock
}

func checkinHistoryRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"checkin_date", "reward_cents", "created_at", "snapshot"})
}
func expectCheckinMetrics(mock sqlmock.Sqlmock, now time.Time) {
	start := checkinDay(now).AddDate(0, 0, -6)
	mock.ExpectQuery("SELECT COUNT").WithArgs(int64(42), start, now).WillReturnRows(sqlmock.NewRows([]string{"active", "spend"}).AddRow(5, 3.2))
	mock.ExpectQuery("SELECT EXISTS").WithArgs(int64(42), start, now).WillReturnRows(sqlmock.NewRows([]string{"recharge"}).AddRow(true))
}
func expectCheckinLock(mock sqlmock.Sqlmock) {
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM users.*FOR UPDATE").WithArgs(int64(42)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(42))
}

func TestCheckinStreakBreakAndWindow(t *testing.T) {
	s, mock := newCheckinTestService(t)
	// Yesterday exists, the day before is missing; older claims must not extend the streak.
	mock.ExpectQuery("SELECT checkin_date::text").WithArgs(int64(42), "2026-09-18").WillReturnRows(checkinHistoryRows().AddRow("2026-09-17", 3, s.now(), `{}`).AddRow("2026-09-15", 1, s.now(), `{}`))
	expectCheckinMetrics(mock, s.now())
	status, err := s.Status(t.Context(), 42)
	require.NoError(t, err)
	require.Equal(t, 2, status.Snapshot.StreakDays)
	require.False(t, status.CheckedIn)
	require.Equal(t, "2026-09-19T00:00:00+08:00", status.NextResetAt.Format(time.RFC3339))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCheckinClaimAtomicAndRetry(t *testing.T) {
	s, mock := newCheckinTestService(t)
	expectCheckinLock(mock)
	mock.ExpectQuery("SELECT checkin_date::text").WillReturnRows(checkinHistoryRows())
	expectCheckinMetrics(mock, s.now())
	mock.ExpectQuery("INSERT INTO daily_checkins").WithArgs(int64(42), "2026-09-18", 1, sqlmock.AnyArg(), s.now(), checkinTestHash).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(101))
	mock.ExpectExec(`UPDATE users SET balance=balance\+`).WithArgs(int64(42), 1, s.now()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO redeem_codes").WithArgs("CHECKIN-101", 1, int64(42), s.now()).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	first, err := s.Claim(t.Context(), 42, checkinTestHash)
	require.NoError(t, err)
	require.True(t, first.CheckedIn)
	require.Equal(t, 1, first.RewardCents)
	raw, err := json.Marshal(first.Snapshot)
	require.NoError(t, err)
	s.draw = func(int64) (int64, error) { t.Fatal("retry must not redraw"); return 0, nil }
	expectCheckinLock(mock)
	mock.ExpectQuery("SELECT checkin_date::text").WillReturnRows(checkinHistoryRows().AddRow("2026-09-18", 1, s.now(), raw))
	mock.ExpectCommit()
	retry, err := s.Claim(t.Context(), 42, checkinTestHash)
	require.NoError(t, err)
	require.Equal(t, first, retry)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCheckinLedgerFailureRollsBackCredit(t *testing.T) {
	s, mock := newCheckinTestService(t)
	expectCheckinLock(mock)
	mock.ExpectQuery("SELECT checkin_date::text").WillReturnRows(checkinHistoryRows())
	expectCheckinMetrics(mock, s.now())
	mock.ExpectQuery("INSERT INTO daily_checkins").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(101))
	mock.ExpectExec("UPDATE users").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO redeem_codes").WillReturnError(errors.New("ledger unavailable"))
	mock.ExpectRollback()
	_, err := s.Claim(t.Context(), 42, checkinTestHash)
	require.ErrorContains(t, err, "ledger unavailable")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCheckinRejectsInactiveAndSimpleMode(t *testing.T) {
	s, mock := newCheckinTestService(t)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM users").WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()
	_, err := s.Claim(t.Context(), 42, checkinTestHash)
	require.ErrorIs(t, err, ErrUserNotFound)
	s.cfg = &config.Config{RunMode: config.RunModeSimple}
	_, err = s.Claim(t.Context(), 42, checkinTestHash)
	require.Error(t, err)
	_, err = s.Status(t.Context(), 42)
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

type checkinSwitchStub bool

func (s checkinSwitchStub) IsCheckinEnabled(context.Context) bool { return bool(s) }

func TestCheckinRejectsWhenSwitchedOff(t *testing.T) {
	s, mock := newCheckinTestService(t)
	s.settings = checkinSwitchStub(false)
	_, err := s.Claim(t.Context(), 42, checkinTestHash)
	require.ErrorIs(t, err, ErrCheckinDisabled)
	_, err = s.Status(t.Context(), 42)
	require.ErrorIs(t, err, ErrCheckinDisabled)
	s.settings = nil
	_, err = s.Status(t.Context(), 42)
	require.ErrorIs(t, err, ErrCheckinDisabled)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCheckinRejectsMissingFingerprintBeforeDatabase(t *testing.T) {
	s, mock := newCheckinTestService(t)
	for _, hash := range []string{"", "invented"} {
		_, err := s.Claim(t.Context(), 42, hash)
		require.ErrorIs(t, err, ErrCheckinCaptchaRequired)
	}
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCheckinRejectsReusedCaptchaBeforeCredit(t *testing.T) {
	s, mock := newCheckinTestService(t)
	expectCheckinLock(mock)
	mock.ExpectQuery("SELECT checkin_date::text").WillReturnRows(checkinHistoryRows())
	expectCheckinMetrics(mock, s.now())
	mock.ExpectQuery("INSERT INTO daily_checkins").WillReturnError(&pq.Error{Code: "23505", Constraint: "daily_checkins_captcha_hash_unique"})
	mock.ExpectRollback()
	_, err := s.Claim(t.Context(), 42, checkinTestHash)
	require.ErrorIs(t, err, ErrCheckinCaptchaRequired)
	require.NoError(t, mock.ExpectationsWereMet())
}
