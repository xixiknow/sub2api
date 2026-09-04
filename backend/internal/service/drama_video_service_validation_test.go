package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseDramaVideoCreatePayloadNormalizesCompatFields(t *testing.T) {
	payload, err := parseDramaVideoCreatePayload([]byte(`{
		"model":"seedance2.0",
		"prompt":"@图1 with @视频1 and @音频1",
		"duration":5,
		"ratio":"16:9",
		"referenceImages":["https://example.com/image.png"],
		"referenceVideos":["https://example.com/video.mp4"],
		"referenceAudios":["https://example.com/audio.mp3"]
	}`))
	require.NoError(t, err)
	require.NotNil(t, payload.Seconds)
	require.Equal(t, 5, *payload.Seconds)
	require.Equal(t, "16:9", payload.AspectRatio)
	require.Len(t, payload.References, 3)
}

func TestParseDramaVideoCreatePayloadRejectsMixedFirstLastFrameMode(t *testing.T) {
	_, err := parseDramaVideoCreatePayload([]byte(`{
		"model":"seedance2.0",
		"prompt":"@图1 @图2",
		"aspect_ratio":"16:9",
		"first_image":"https://example.com/start.png",
		"last_image":"https://example.com/end.png"
	}`))
	require.Error(t, err)
}

func TestParseDramaVideoCreatePayloadAcceptsLowercaseAutoAspectRatio(t *testing.T) {
	payload, err := parseDramaVideoCreatePayload([]byte(`{
		"model":"seedance2.0",
		"prompt":"@图1 @图2",
		"aspect_ratio":"auto",
		"first_image":"https://example.com/start.png",
		"last_image":"https://example.com/end.png"
	}`))
	require.NoError(t, err)
	require.Equal(t, "Auto", payload.AspectRatio)
}
