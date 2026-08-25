package media

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestFFmpegPipelineIntegration(t *testing.T) {
	if os.Getenv("VIDEOHUB_FFMPEG_INTEGRATION") != "1" {
		t.Skip("set VIDEOHUB_FFMPEG_INTEGRATION=1 to run real ffmpeg tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	tests := []struct {
		name       string
		inputSize  string
		wantWidth  int
		wantHeight int
	}{
		{name: "downscale above 1080p", inputSize: "2560x1440", wantWidth: 1920, wantHeight: 1080},
		{name: "do not upscale small video", inputSize: "640x360", wantWidth: 640, wantHeight: 360},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			workDir := t.TempDir()
			inputPath := filepath.Join(workDir, "input.mkv")
			outputPath := filepath.Join(workDir, "playback.mp4")
			generateSyntheticVideo(t, ctx, inputPath, test.inputSize)

			inputInfo, err := Probe(ctx, inputPath)
			if err != nil {
				t.Fatalf("Probe(input): %v", err)
			}
			if err := Transcode(ctx, inputPath, outputPath, inputInfo.DurationMillis, nil); err != nil {
				t.Fatalf("Transcode(): %v", err)
			}
			outputInfo, err := Probe(ctx, outputPath)
			if err != nil {
				t.Fatalf("Probe(output): %v", err)
			}
			if outputInfo.Width != test.wantWidth || outputInfo.Height != test.wantHeight {
				t.Fatalf("output size = %dx%d, want %dx%d", outputInfo.Width, outputInfo.Height, test.wantWidth, test.wantHeight)
			}
			if outputInfo.VideoCodec != "h264" || outputInfo.AudioCodec != "aac" {
				t.Fatalf("output codecs = %s/%s, want h264/aac", outputInfo.VideoCodec, outputInfo.AudioCodec)
			}

			covers, err := GenerateCandidateCovers(ctx, inputPath, filepath.Join(workDir, "covers"), inputInfo.DurationMillis)
			if err != nil {
				t.Fatalf("GenerateCandidateCovers(): %v", err)
			}
			if len(covers) != 3 {
				t.Fatalf("generated %d covers, want 3", len(covers))
			}
			for _, cover := range covers {
				stat, err := os.Stat(cover)
				if err != nil {
					t.Fatalf("cover %s: %v", cover, err)
				}
				if stat.Size() == 0 {
					t.Fatalf("cover %s is empty", cover)
				}
			}
		})
	}
}

func generateSyntheticVideo(t *testing.T, ctx context.Context, outputPath, size string) {
	t.Helper()
	cmd := exec.CommandContext(
		ctx,
		"ffmpeg",
		"-y",
		"-v", "error",
		"-f", "lavfi",
		"-i", fmt.Sprintf("testsrc=size=%s:rate=10", size),
		"-f", "lavfi",
		"-i", "sine=frequency=1000:sample_rate=44100",
		"-t", "1",
		"-c:v", "libx264",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		outputPath,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate synthetic video: %v: %s", err, string(output))
	}
}
