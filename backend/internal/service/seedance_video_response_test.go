package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSeedanceVideoTaskResponsePublicFields(t *testing.T) {
	task := &SeedanceVideoTaskResponse{
		ID:       "task_123",
		Status:   "queued",
		Metadata: &SeedanceVideoTaskMetadata{URL: "https://example.com/content.mp4"},
	}

	require.Equal(t, "task_123", task.PublicID())
	require.Equal(t, "https://example.com/content.mp4", task.ContentURL())
}

func TestSeedanceVideoTaskResponseFallbackFields(t *testing.T) {
	task := &SeedanceVideoTaskResponse{
		TaskID: "task_456",
		Result: &SeedanceVideoTaskResult{VideoURL: "https://example.com/fallback.mp4"},
	}

	require.Equal(t, "task_456", task.PublicID())
	require.Equal(t, "https://example.com/fallback.mp4", task.ContentURL())
}

func TestSeedanceVideoTaskCompletionRequiresCompletedStatusAndURL(t *testing.T) {
	completedAt := int64(1787564122)
	require.False(t, seedanceVideoTaskIsCompleted(&SeedanceVideoTaskResponse{
		ID:          "task_queued",
		Status:      "queued",
		CompletedAt: &completedAt,
		Metadata:    &SeedanceVideoTaskMetadata{URL: "https://example.com/proxy.mp4"},
	}))
	require.False(t, seedanceVideoTaskIsCompleted(&SeedanceVideoTaskResponse{
		ID:     "task_no_url",
		Status: "completed",
	}))
	require.True(t, seedanceVideoTaskIsCompleted(&SeedanceVideoTaskResponse{
		ID:       "task_done",
		Status:   "completed",
		Metadata: &SeedanceVideoTaskMetadata{URL: "https://example.com/video.mp4"},
	}))
}

func TestSeedanceVideoCaptureRequestID(t *testing.T) {
	require.Equal(t, BatchImageHoldRequestID("video_123"), SeedanceVideoHoldRequestID(" video_123 "))
	require.Equal(t, "seedance_video_capture:video_123", SeedanceVideoCaptureRequestID(" video_123 "))
	require.Equal(t, "seedance_video_release:video_123", SeedanceVideoReleaseRequestID(" video_123 "))
}

func TestNewSeedanceVideoTaskIDUsesLocalPrefix(t *testing.T) {
	id, err := NewSeedanceVideoTaskID()
	require.NoError(t, err)
	require.Regexp(t, `^video_[a-f0-9]{32}$`, id)
	require.True(t, shouldCaptureSeedanceHold(&DramaVideoTask{TaskID: id}))
	require.False(t, shouldCaptureSeedanceHold(&DramaVideoTask{TaskID: "task_123"}))
}

func TestExposeSeedanceLocalTaskID(t *testing.T) {
	task := &SeedanceVideoTaskResponse{
		ID:     "task_upstream",
		TaskID: "task_upstream",
	}
	exposeSeedanceLocalTaskID(task, " video_local ")

	require.Equal(t, "video_local", task.ID)
	require.Equal(t, "video_local", task.TaskID)
}
