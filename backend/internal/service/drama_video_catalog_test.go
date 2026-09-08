package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildDramaVideoCatalog(t *testing.T) {
	catalog := BuildDramaVideoCatalog()
	require.Len(t, catalog.Families, 10)
	require.Equal(t, DramaVideoPublicFamilies(), catalogFamilyNames(catalog))

	byName := map[string]DramaVideoCatalogFamily{}
	for _, family := range catalog.Families {
		byName[family.Family] = family
		require.NotEmpty(t, family.Prices, family.Family)
		require.NotEmpty(t, family.CreatePath, family.Family)
		defaults := DefaultDramaVideoModelPrices()[family.Family]
		require.Len(t, family.Prices, len(defaults), family.Family)
		for _, price := range family.Prices {
			_, supported := dramaVideoCapabilities[family.Family].resolutions[price.Resolution]
			require.True(t, supported, "%s %s", family.Family, price.Resolution)
			require.Equal(t, defaults[price.Resolution], price.Price)
		}
	}

	a := byName[DramaFamilySeedance20A]
	require.Equal(t, DramaVideoBillingPerSecond, a.BillingUnit)
	require.Equal(t, DramaVideoCreatePathVideos, a.CreatePath)
	require.Equal(t, []string{"480p", "720p", "1080p"}, catalogResolutions(a))
	require.NotContains(t, catalogResolutions(a), VideoBillingResolution4K)

	c := byName[DramaFamilySeedance20C]
	require.Equal(t, DramaVideoBillingPerClip, c.BillingUnit)
	require.Equal(t, []string{"720p"}, catalogResolutions(c))
	require.True(t, c.DurationRequired)
	require.InDelta(t, 3.5, c.Prices[0].Price, 0.0001)

	b := byName[DramaFamilySeedance25B]
	require.Equal(t, 30, b.FixedDuration)
	require.Equal(t, []string{"720p"}, catalogResolutions(b))
	require.False(t, b.AllowAudio)
}

func catalogFamilyNames(catalog DramaVideoCatalog) []string {
	names := make([]string, 0, len(catalog.Families))
	for _, family := range catalog.Families {
		names = append(names, family.Family)
	}
	return names
}

func catalogResolutions(family DramaVideoCatalogFamily) []string {
	out := make([]string, 0, len(family.Prices))
	for _, price := range family.Prices {
		out = append(out, price.Resolution)
	}
	return out
}
