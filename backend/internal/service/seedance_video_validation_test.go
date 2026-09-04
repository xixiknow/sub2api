package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseSeedanceVideoTaskPayloadNormalizesAliases(t *testing.T) {
	payload, err := parseSeedanceVideoTaskPayload([]byte(`{
		"channel":"s",
		"model":"seedance-2.0-fast-0826",
		"prompt":"move forward",
		"duration":5,
		"ratio":"16:9",
		"task_mode":"references",
		"materials":[
			{"upload_id":"upl_image_1","role":"reference_image"},
			{"upload_id":"upl_video_1","role":"reference_video"},
			{"upload_id":"upl_audio_1","role":"reference_audio"}
		]
	}`))
	require.NoError(t, err)
	require.NotNil(t, payload.Seconds)
	require.Equal(t, 5, *payload.Seconds)
	require.Equal(t, "16:9", payload.AspectRatio)
	require.Equal(t, "s", payload.Channel)
	require.Equal(t, "seedance-2.0-fast-0826", payload.Model)
	require.Len(t, payload.Materials, 3)
}

func TestParseSeedanceVideoTaskPayloadAcceptsMappedAlias(t *testing.T) {
	payload, err := parseSeedanceVideoTaskPayload([]byte(`{
		"channel":"s",
		"model":"seedance-2.0-fast",
		"prompt":"move forward",
		"duration":5,
		"ratio":"16:9",
		"task_mode":"text"
	}`))
	require.NoError(t, err)
	require.Equal(t, "seedance-2.0-fast", payload.Model)
}

func TestResolveSeedanceVideoModelAppliesAccountMapping(t *testing.T) {
	account := &Account{
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"seedance-2.0-fast": "seedance-2.0-fast-0826",
			},
		},
	}

	require.Equal(t, SeedanceVideoModel0826Fast, resolveSeedanceVideoModel(account, "seedance-2.0-fast"))
}

func TestValidateSeedanceVideoTaskModelEnforcesFastResolutionLimits(t *testing.T) {
	require.NoError(t, validateSeedanceVideoTaskModel(SeedanceVideoModel0826Fast, "720p"))
	require.Error(t, validateSeedanceVideoTaskModel(SeedanceVideoModel0826Fast, "1080p"))
}

func TestParseSeedanceVideoTaskPayloadRejectsBadMaterialRoles(t *testing.T) {
	_, err := parseSeedanceVideoTaskPayload([]byte(`{
		"channel":"s",
		"model":"seedance-2.0-0826",
		"prompt":"move forward",
		"seconds":5,
		"task_mode":"first_last_frame",
		"materials":[
			{"upload_id":"upl_image_1","role":"first_frame"},
			{"upload_id":"upl_image_2","role":"reference_image"}
		]
	}`))
	require.Error(t, err)
}
