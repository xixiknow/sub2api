package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNormalizeAffiliateLevelDetails_FixedLevelsAndMaskedEmails(t *testing.T) {
	parentID := int64(7)
	joinedAt := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	lastRebateAt := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)

	out := normalizeAffiliateLevelDetails([]AffiliateLevelDetail{
		{
			Level:           2,
			InviteeCount:    1,
			TotalRebate:     5,
			FrozenRebate:    2,
			AvailableRebate: 3,
			Invitees: []AffiliateLevelInvitee{
				{
					UserID:          9,
					Email:           "invitee@example.com",
					Username:        "invitee",
					JoinedAt:        &joinedAt,
					TotalRebate:     5,
					FrozenRebate:    2,
					AvailableRebate: 3,
					OrderCount:      2,
					LastRebateAt:    &lastRebateAt,
					ParentUserID:    &parentID,
					ParentEmail:     "parent@example.com",
					ParentUsername:  "parent",
				},
			},
		},
		{Level: 99, Invitees: []AffiliateLevelInvitee{{Email: "ignored@example.com"}}},
	})

	require.Len(t, out, AffiliateLevelsMax)
	require.Equal(t, 1, out[0].Level)
	require.Empty(t, out[0].Invitees)
	require.Equal(t, 2, out[1].Level)
	require.Equal(t, 3, out[2].Level)

	require.Equal(t, 1, out[1].InviteeCount)
	require.InDelta(t, 5, out[1].TotalRebate, 1e-9)
	require.InDelta(t, 2, out[1].FrozenRebate, 1e-9)
	require.InDelta(t, 3, out[1].AvailableRebate, 1e-9)
	require.Len(t, out[1].Invitees, 1)
	require.Equal(t, "i***@e***.com", out[1].Invitees[0].Email)
	require.Equal(t, "p***@e***.com", out[1].Invitees[0].ParentEmail)
	require.Equal(t, parentID, *out[1].Invitees[0].ParentUserID)
	require.Equal(t, joinedAt, *out[1].Invitees[0].JoinedAt)
	require.Equal(t, lastRebateAt, *out[1].Invitees[0].LastRebateAt)
}
