package service

import (
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestGrowthServiceGetStatusUsesCommunityPromoAndTutorialEvent(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	userID := int64(12)
	communityAt := time.Date(2026, 5, 13, 10, 0, 0, 0, time.UTC)
	tutorialAt := time.Date(2026, 5, 13, 10, 5, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT pcu.used_at").
		WithArgs(userID, PromoCodePurposeCommunityJoin).
		WillReturnRows(sqlmock.NewRows([]string{"used_at"}).AddRow(communityAt))
	mock.ExpectQuery("SELECT created_at").
		WithArgs(userID, GrowthEventAffiliateTutorialDone).
		WillReturnRows(sqlmock.NewRows([]string{"created_at"}).AddRow(tutorialAt))
	mock.ExpectQuery("SELECT COALESCE\\(aff_count, 0\\)").
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"aff_count"}).AddRow(0))
	expectBadgeRefresh(mock, userID)

	status, err := newGrowthServiceWithDB(db).GetStatus(t.Context(), userID)
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if !status.CommunityJoined || status.CommunityJoinedAt == nil || !status.CommunityJoinedAt.Equal(communityAt) {
		t.Fatalf("CommunityJoined = (%v, %v), want true at %v", status.CommunityJoined, status.CommunityJoinedAt, communityAt)
	}
	if !status.AffiliateTutorialDone || status.AffiliateTutorialDoneAt == nil || !status.AffiliateTutorialDoneAt.Equal(tutorialAt) {
		t.Fatalf("AffiliateTutorialDone = (%v, %v), want true at %v", status.AffiliateTutorialDone, status.AffiliateTutorialDoneAt, tutorialAt)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestGrowthServiceGetStatusTreatsExistingInviteAsTutorialDone(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	userID := int64(42)

	mock.ExpectQuery("SELECT pcu.used_at").
		WithArgs(userID, PromoCodePurposeCommunityJoin).
		WillReturnRows(sqlmock.NewRows([]string{"used_at"}))
	mock.ExpectQuery("SELECT created_at").
		WithArgs(userID, GrowthEventAffiliateTutorialDone).
		WillReturnRows(sqlmock.NewRows([]string{"created_at"}))
	mock.ExpectQuery("SELECT COALESCE\\(aff_count, 0\\)").
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"aff_count"}).AddRow(1))
	expectBadgeRefresh(mock, userID)

	status, err := newGrowthServiceWithDB(db).GetStatus(t.Context(), userID)
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if status.CommunityJoined {
		t.Fatal("CommunityJoined = true, want false")
	}
	if !status.AffiliateTutorialDone {
		t.Fatal("AffiliateTutorialDone = false, want true for existing invitee count")
	}
	if status.AffiliateTutorialDoneAt != nil {
		t.Fatalf("AffiliateTutorialDoneAt = %v, want nil for invite-count fallback", status.AffiliateTutorialDoneAt)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestGrowthServiceMarkAffiliateTutorialDoneIsIdempotent(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	userID := int64(7)
	tutorialAt := time.Date(2026, 5, 13, 11, 0, 0, 0, time.UTC)

	mock.ExpectExec("INSERT INTO user_growth_events").
		WithArgs(userID, GrowthEventAffiliateTutorialDone).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT pcu.used_at").
		WithArgs(userID, PromoCodePurposeCommunityJoin).
		WillReturnRows(sqlmock.NewRows([]string{"used_at"}))
	mock.ExpectQuery("SELECT created_at").
		WithArgs(userID, GrowthEventAffiliateTutorialDone).
		WillReturnRows(sqlmock.NewRows([]string{"created_at"}).AddRow(tutorialAt))
	mock.ExpectQuery("SELECT COALESCE\\(aff_count, 0\\)").
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"aff_count"}).AddRow(0))
	expectBadgeRefresh(mock, userID)

	status, err := newGrowthServiceWithDB(db).MarkAffiliateTutorialDone(t.Context(), userID)
	if err != nil {
		t.Fatalf("MarkAffiliateTutorialDone: %v", err)
	}
	if !status.AffiliateTutorialDone {
		t.Fatal("AffiliateTutorialDone = false, want true")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestGrowthServiceStarterTaskGateStatusSupportsOldUserData(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	userID := int64(88)
	mock.ExpectQuery("SELECT\\s+EXISTS").
		WithArgs(userID, GrowthEventAffiliateTutorialDone).
		WillReturnRows(sqlmock.NewRows([]string{
			"has_api_key",
			"has_first_request",
			"affiliate_tutorial_done",
		}).AddRow(true, true, true))

	status, err := newGrowthServiceWithDB(db).starterTaskGateStatus(t.Context(), userID)
	if err != nil {
		t.Fatalf("starterTaskGateStatus: %v", err)
	}
	if !status.communityWelfareReady() {
		t.Fatalf("communityWelfareReady = false, want true: %+v", status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestGrowthServiceStarterTaskGateStatusBlocksIncompleteUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	userID := int64(89)
	mock.ExpectQuery("SELECT\\s+EXISTS").
		WithArgs(userID, GrowthEventAffiliateTutorialDone).
		WillReturnRows(sqlmock.NewRows([]string{
			"has_api_key",
			"has_first_request",
			"affiliate_tutorial_done",
		}).AddRow(true, false, true))

	status, err := newGrowthServiceWithDB(db).starterTaskGateStatus(t.Context(), userID)
	if err != nil {
		t.Fatalf("starterTaskGateStatus: %v", err)
	}
	if status.communityWelfareReady() {
		t.Fatalf("communityWelfareReady = true, want false: %+v", status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func expectBadgeRefresh(mock sqlmock.Sqlmock, userID int64) {
	mock.ExpectQuery("WITH metrics").
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"refreshed", "removed"}).AddRow(0, 0))
	mock.ExpectQuery("SELECT badge_id, unlocked_at").
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"badge_id", "unlocked_at"}))
}
