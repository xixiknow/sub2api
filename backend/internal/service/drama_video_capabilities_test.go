package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCanonicalDramaVideoPriceFamily(t *testing.T) {
	require.Equal(t, DramaFamilySeedance20F, CanonicalDramaVideoPriceFamily("seedance2.0-F-1080p"))
	require.Equal(t, DramaFamilySeedance20F, CanonicalDramaVideoPriceFamily("seedance2.0-F-720p"))
	require.Equal(t, DramaFamilySeedance20E, CanonicalDramaVideoPriceFamily("seedance2.0-E-720p"))
	require.Equal(t, DramaFamilySeedance20FF, CanonicalDramaVideoPriceFamily("seedance2.0-fast-F-720p"))
	require.Equal(t, DramaFamilySeedance25B, CanonicalDramaVideoPriceFamily("sd25-30s"))
	require.Equal(t, DramaFamilyMinimaxH3, CanonicalDramaVideoPriceFamily("minimax-h3"))
	require.Empty(t, CanonicalDramaVideoPriceFamily("sd2-5-vref-720p"))
	require.Empty(t, CanonicalDramaVideoPriceFamily("seedance2.0-E-1080p"))
}

func TestResolveDramaVideoModelAliasAndResolution(t *testing.T) {
	resolved, err := resolveDramaVideoModel("seedance2.0-F-1080p", "")
	require.NoError(t, err)
	require.Equal(t, DramaFamilySeedance20F, resolved.Family)
	require.Equal(t, VideoBillingResolution1080P, resolved.Resolution)
	require.Equal(t, "seedance2.0-F-1080p", resolved.UpstreamModel)
	require.Equal(t, DramaVideoCreatePathGens, resolved.CreatePath)
	require.Equal(t, DramaVideoBillingPerClip, resolved.BillingUnit)

	_, err = resolveDramaVideoModel("seedance2.0-F-720p", "1080p")
	require.Error(t, err)

	_, err = resolveDramaVideoModel("seedance2.0-A", "4k")
	require.Error(t, err)

	resolved, err = resolveDramaVideoModel("seedance2.0-B", "4k")
	require.NoError(t, err)
	require.Equal(t, VideoBillingResolution4K, resolved.Resolution)
}

func TestParseDramaVideoCreatePayloadPathSplit(t *testing.T) {
	aBody := []byte(`{"model":"seedance2.0-A","prompt":"hello","seconds":4}`)
	_, _, err := parseDramaVideoCreatePayload(aBody, dramaVideoSurfaceGens)
	require.Error(t, err)

	bBody := []byte(`{"model":"seedance2.0-B","prompt":"hello","seconds":8}`)
	_, _, err = parseDramaVideoCreatePayload(bBody, dramaVideoSurfaceVideos)
	require.Error(t, err)

	_, resolved, err := parseDramaVideoCreatePayload(bBody, dramaVideoSurfaceGens)
	require.NoError(t, err)
	require.Equal(t, DramaFamilySeedance20B, resolved.Family)
}

func TestParseDramaVideoCreatePayloadRejectsSESeriesFields(t *testing.T) {
	body := []byte(`{"model":"seedance2.0-E","prompt":"hello","seconds":8,"generate_audio":true}`)
	_, _, err := parseDramaVideoCreatePayload(body, dramaVideoSurfaceGens)
	require.Error(t, err)
}

func TestParseDramaVideoCreatePayloadRequiresAPlaceholders(t *testing.T) {
	body := []byte(`{
		"model":"seedance2.0-A",
		"prompt":"a scene",
		"seconds":4,
		"references":[{"type":"image","role":"reference","source":"https://example.com/a.png"}]
	}`)
	_, _, err := parseDramaVideoCreatePayload(body, dramaVideoSurfaceVideos)
	require.Error(t, err)

	okBody := []byte(`{
		"model":"seedance2.0-A",
		"prompt":"use @图1",
		"seconds":4,
		"references":[{"type":"image","role":"reference","source":"https://example.com/a.png"}]
	}`)
	payload, _, err := parseDramaVideoCreatePayload(okBody, dramaVideoSurfaceVideos)
	require.NoError(t, err)
	require.Len(t, payload.References, 1)
}

func TestParseDramaVideoCreatePayloadDurationAndResolution(t *testing.T) {
	_, _, err := parseDramaVideoCreatePayload([]byte(`{"model":"seedance2.0-A","prompt":"hello","seconds":3}`), dramaVideoSurfaceVideos)
	require.Error(t, err)
	_, _, err = parseDramaVideoCreatePayload([]byte(`{"model":"seedance-2.0-C","prompt":"hello"}`), dramaVideoSurfaceVideos)
	require.Error(t, err)
	_, resolved, err := parseDramaVideoCreatePayload([]byte(`{"model":"seedance-2.5-B","prompt":"hello","seconds":30}`), dramaVideoSurfaceVideos)
	require.NoError(t, err)
	require.Equal(t, DramaFamilySeedance25B, resolved.Family)
	require.Equal(t, VideoBillingResolution720P, resolved.Resolution)

	_, _, err = parseDramaVideoCreatePayload([]byte(`{"model":"seedance2.0-fast-A","prompt":"hello","resolution":"720p"}`), dramaVideoSurfaceVideos)
	require.Error(t, err)
	_, _, err = parseDramaVideoCreatePayload([]byte(`{"model":"minimax-h3","prompt":"hello","aspect_ratio":"1:1"}`), dramaVideoSurfaceVideos)
	require.Error(t, err)
}

func TestParseDramaVideoCreatePayloadAllowsCGenerateAudio(t *testing.T) {
	_, resolved, err := parseDramaVideoCreatePayload([]byte(`{"model":"seedance-2.0-C","prompt":"hello","seconds":8,"generate_audio":true}`), dramaVideoSurfaceVideos)
	require.NoError(t, err)
	require.Equal(t, DramaFamilySeedance20C, resolved.Family)
	require.Equal(t, DramaVideoBillingPerClip, resolved.BillingUnit)
}

func TestParseDramaVideoCreatePayloadFirstLastAndMedia(t *testing.T) {
	_, _, err := parseDramaVideoCreatePayload([]byte(`{
		"model":"seedance2.0-A",
		"prompt":"use @图1 and @图2",
		"seconds":4,
		"aspect_ratio":"16:9",
		"first_frame":"https://example.com/a.png",
		"last_frame":"https://example.com/b.png"
	}`), dramaVideoSurfaceVideos)
	require.Error(t, err)

	payload, _, err := parseDramaVideoCreatePayload([]byte(`{
		"model":"seedance2.0-A",
		"prompt":"use @图1 and @图2",
		"seconds":4,
		"aspect_ratio":"auto",
		"first_frame":"https://example.com/a.png",
		"last_frame":"https://example.com/b.png"
	}`), dramaVideoSurfaceVideos)
	require.NoError(t, err)
	require.True(t, hasDramaFirstLast(payload.References))

	_, _, err = parseDramaVideoCreatePayload([]byte(`{
		"model":"seedance2.0-F",
		"prompt":"hello",
		"seconds":8,
		"references":[{"type":"video","role":"reference","source":"https://example.com/v.mp4"}]
	}`), dramaVideoSurfaceGens)
	require.Error(t, err)
}

func TestMarshalDramaVideoUpstreamBodyUsesIndependentName(t *testing.T) {
	payload := dramaVideoCreatePayload{
		Model:       DramaFamilySeedance20F,
		Prompt:      "hello",
		Seconds:     json.RawMessage("8"),
		Resolution:  VideoBillingResolution1080P,
		AspectRatio: "16:9",
	}
	body, err := marshalDramaVideoUpstreamBody(payload, "seedance2.0-F-1080p")
	require.NoError(t, err)
	require.Contains(t, string(body), `"model":"seedance2.0-F-1080p"`)
}
