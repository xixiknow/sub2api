//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestSettingService_ImageWorkbenchEnabled_DefaultsOff(t *testing.T) {
	svc := NewSettingService(&settingPublicRepoStub{values: map[string]string{}}, &config.Config{})

	public, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.False(t, public.ImageWorkbenchEnabled)
	require.False(t, svc.parseSettings(map[string]string{}).ImageWorkbenchEnabled)
}

func TestSettingService_ImageWorkbenchEnabled_ExposedWhenOn(t *testing.T) {
	repo := &settingPublicRepoStub{values: map[string]string{SettingKeyImageWorkbenchEnabled: "true"}}
	svc := NewSettingService(repo, &config.Config{})

	public, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.True(t, public.ImageWorkbenchEnabled)
	require.True(t, svc.parseSettings(map[string]string{SettingKeyImageWorkbenchEnabled: "true"}).ImageWorkbenchEnabled)
}
