//go:build unit

package admin

import (
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestRechargeCardSettingsSaveAndPartialUpdate(t *testing.T) {
	h, repo := newStepUpSwitchTestHandler(t, nil)
	rec := doUpdateSettings(t, h, map[string]any{
		"recharge_card_enabled":   true,
		"recharge_card_url":       " https://shop.example.com/#/cards ",
		"recharge_card_open_mode": "iframe",
	}, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "https://shop.example.com/#/cards", repo.values[service.SettingKeyRechargeCardURL])
	require.Contains(t, rec.Body.String(), `"recharge_card_enabled":true`)
	rec = doUpdateSettings(t, h, map[string]any{"recharge_card_open_mode": "new_tab"}, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "true", repo.values[service.SettingKeyRechargeCardEnabled])
	require.Equal(t, "https://shop.example.com/#/cards", repo.values[service.SettingKeyRechargeCardURL])
	require.Equal(t, "new_tab", repo.values[service.SettingKeyRechargeCardOpenMode])
	rec = doUpdateSettings(t, h, map[string]any{"site_name": "Card Site"}, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "new_tab", repo.values[service.SettingKeyRechargeCardOpenMode])
	rec = doUpdateSettings(t, h, map[string]any{"recharge_card_enabled": false}, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "false", repo.values[service.SettingKeyRechargeCardEnabled])
}

func TestRechargeCardSettingsRejectInvalidValues(t *testing.T) {
	for _, payload := range []map[string]any{
		{"recharge_card_enabled": true},
		{"recharge_card_url": "javascript:alert(1)"},
		{"recharge_card_url": "/relative"},
		{"recharge_card_url": "https://user:secret@shop.example.com"},
		{"recharge_card_open_mode": "popup"},
		{"recharge_card_open_mode": ""},
	} {
		h, _ := newStepUpSwitchTestHandler(t, nil)
		rec := doUpdateSettings(t, h, payload, nil)
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}
