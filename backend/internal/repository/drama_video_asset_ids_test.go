package repository

import (
	"testing"

	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

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
