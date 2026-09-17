//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestRechargeCardPublicSettingsAndFrameOrigins(t *testing.T) {
	ctx := context.Background()
	repo := &settingPublicRepoStub{values: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{})
	settings, err := svc.GetPublicSettings(ctx)
	require.NoError(t, err)
	require.False(t, settings.RechargeCardEnabled)
	require.Equal(t, "iframe", settings.RechargeCardOpenMode)
	repo.values[SettingKeyRechargeCardEnabled] = "true"
	repo.values[SettingKeyRechargeCardURL] = "https://shop.example.com/cards"
	settings, err = svc.GetPublicSettings(ctx)
	require.NoError(t, err)
	require.True(t, settings.RechargeCardEnabled)
	require.Equal(t, "https://shop.example.com/cards", settings.RechargeCardURL)
	raw, err := svc.GetPublicSettingsForInjection(ctx)
	require.NoError(t, err)
	injected := raw.(*PublicSettingsInjectionPayload)
	require.True(t, injected.RechargeCardEnabled)
	require.Equal(t, settings.RechargeCardURL, injected.RechargeCardURL)
	require.Equal(t, "iframe", injected.RechargeCardOpenMode)
	origins, err := svc.GetFrameSrcOrigins(ctx)
	require.NoError(t, err)
	require.Contains(t, origins, "https://shop.example.com")
	repo.values[SettingKeyRechargeCardOpenMode] = "new_tab"
	origins, err = svc.GetFrameSrcOrigins(ctx)
	require.NoError(t, err)
	require.NotContains(t, origins, "https://shop.example.com")
	repo.values[SettingKeyRechargeCardOpenMode] = "iframe"
	repo.values[SettingKeyRechargeCardEnabled] = "false"
	origins, err = svc.GetFrameSrcOrigins(ctx)
	require.NoError(t, err)
	require.NotContains(t, origins, "https://shop.example.com")
}
