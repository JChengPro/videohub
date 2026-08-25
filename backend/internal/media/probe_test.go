package media

import (
	"errors"
	"testing"
)

func TestParseProbeOutput(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
		"streams": [
			{
				"codec_name": "h264",
				"codec_type": "video",
				"width": 1920,
				"height": 1080
			},
			{
				"codec_name": "aac",
				"codec_type": "audio"
			}
		],
		"format": {
			"format_name": "mov,mp4,m4a,3gp,3g2,mj2",
			"duration": "12.345000"
		}
	}`)

	got, err := parseProbeOutput(raw)
	if err != nil {
		t.Fatalf("parse probe output: %v", err)
	}

	if got.VideoCodec != "h264" {
		t.Fatalf("expected video codec h264, got %q", got.VideoCodec)
	}
	if got.AudioCodec != "aac" {
		t.Fatalf("expected audio codec aac, got %q", got.AudioCodec)
	}
	if got.Width != 1920 || got.Height != 1080 {
		t.Fatalf("expected 1920x1080, got %dx%d", got.Width, got.Height)
	}
	if got.DurationMillis != 12345 {
		t.Fatalf("expected duration 12345 ms, got %d", got.DurationMillis)
	}
	if got.FormatName != "mov,mp4,m4a,3gp,3g2,mj2" {
		t.Fatalf("unexpected format name %q", got.FormatName)
	}
}

func TestParseProbeOutputRejectsAudioOnlyMedia(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
		"streams": [
			{
				"codec_name": "mp3",
				"codec_type": "audio"
			}
		],
		"format": {
			"format_name": "mp3",
			"duration": "8.500000"
		}
	}`)

	_, err := parseProbeOutput(raw)
	if !errors.Is(err, ErrInvalidMedia) {
		t.Fatalf("expected ErrInvalidMedia, got %v", err)
	}
}

func TestParseProbeOutputAppliesVideoRotation(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
		"streams": [
			{
				"codec_name": "h264",
				"codec_type": "video",
				"width": 1920,
				"height": 1080,
				"side_data_list": [
					{
						"rotation": -90
					}
				]
			}
		],
		"format": {
			"format_name": "mov,mp4,m4a,3gp,3g2,mj2",
			"duration": "10.000000"
		}
	}`)

	got, err := parseProbeOutput(raw)
	if err != nil {
		t.Fatalf("parse rotated video: %v", err)
	}

	if got.Width != 1080 || got.Height != 1920 {
		t.Fatalf(
			"expected rotated display size 1080x1920, got %dx%d",
			got.Width,
			got.Height,
		)
	}
}
