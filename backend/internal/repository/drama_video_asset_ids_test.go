package repository

import (
	"testing"

	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestDramaVideoCreateSQLTypesAssetIDsAsTextArray(t *testing.T) {
	require.Contains(t, dramaVideoCreateQuery, "COALESCE($15, ARRAY[]::text[])")
	require.NotContains(t, dramaVideoCreateQuery, `COALESCE($15, '{}')`)
	require.Contains(t, dramaVideoSelectColumns, "COALESCE(asset_ids, ARRAY[]::text[])")
	require.NotContains(t, dramaVideoSelectColumns, "COALESCE(asset_ids, '{}')")
}

func TestDramaVideoAssetIDsNeverSQLNull(t *testing.T) {
	nilVal, err := pq.StringArray(nil).Value()
	require.NoError(t, err)
	require.Nil(t, nilVal)

	got, err := dramaVideoAssetIDs(nil).Value()
	require.NoError(t, err)
	require.Equal(t, "{}", got)

	got, err = dramaVideoAssetIDs([]string{}).Value()
	require.NoError(t, err)
	require.Equal(t, "{}", got)

	got, err = dramaVideoAssetIDs([]string{"vidasset_1"}).Value()
	require.NoError(t, err)
	require.Equal(t, `{"vidasset_1"}`, got)
}

func TestDramaVideoAssetIDsBindsArrayNotPlainText(t *testing.T) {
	bound := any(dramaVideoAssetIDs(nil))
	_, isPlainString := bound.(string)
	require.False(t, isPlainString, "asset_ids must not bind as a plain text string")
	_, ok := bound.(*pq.StringArray)
	require.True(t, ok, "asset_ids must bind through pq.Array([]string)")
}
