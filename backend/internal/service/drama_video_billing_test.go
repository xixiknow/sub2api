package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCalculateDramaVideoCostPerSecondAndPerClip(t *testing.T) {
	billing := NewBillingService(nil, nil)
	perSecond, err := resolveDramaVideoModel(DramaFamilySeedance20A, "720p")
	require.NoError(t, err)
	cost := billing.CalculateDramaVideoCost(perSecond, 10, nil, 1)
	require.NotNil(t, cost)
	require.InDelta(t, 5.0, cost.ActualCost, 0.0001)

	perClip, err := resolveDramaVideoModel(DramaFamilySeedance20F, "1080p")
	require.NoError(t, err)
	clip := billing.CalculateDramaVideoCost(perClip, 15, nil, 1)
	require.NotNil(t, clip)
	require.InDelta(t, 7.0, clip.ActualCost, 0.0001)

	b4k, err := resolveDramaVideoModel(DramaFamilySeedance20B, "4k")
	require.NoError(t, err)
	fourK := billing.CalculateDramaVideoCost(b4k, 8, nil, 1)
	require.NotNil(t, fourK)
	require.InDelta(t, 40.0, fourK.ActualCost, 0.0001)

	perClipC, err := resolveDramaVideoModel(DramaFamilySeedance20C, "720p")
	require.NoError(t, err)
	require.Equal(t, DramaVideoBillingPerClip, perClipC.BillingUnit)
	clipC := billing.CalculateDramaVideoCost(perClipC, 15, nil, 1)
	require.NotNil(t, clipC)
	require.InDelta(t, 3.5, clipC.ActualCost, 0.0001)
}

func TestLookupVideoModelPriceUsesDramaFamily(t *testing.T) {
	prices := map[string]map[string]float64{
		"seedance2.0-F-720p": {"720p": 4.2},
	}
	normalized := NormalizeVideoModelPrices(prices)
	price := LookupVideoModelPrice(normalized, "seedance2.0-F", "720p")
	require.NotNil(t, price)
	require.InDelta(t, 4.2, *price, 0.0001)
	aliased := LookupVideoModelPrice(normalized, "seedance2.0-F-1080p", "720p")
	require.NotNil(t, aliased)
	require.InDelta(t, 4.2, *aliased, 0.0001)
}

func TestGetDefaultDramaVideoPrice(t *testing.T) {
	price, ok := getDefaultDramaVideoPrice("seedance2.0-fast-F-720p", "720p")
	require.True(t, ok)
	require.InDelta(t, 2.8, price, 0.0001)
}
