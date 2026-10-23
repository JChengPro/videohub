package media

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const maxProgressPercent = 99

var ErrProcessing = errors.New("media processing failed")

type ProgressFunc func(percent int)

// ProgressPercent converts ffmpeg's processed microseconds into a bounded percentage.
func ProgressPercent(outTimeMicros, durationMillis int64) int {
	if outTimeMicros <= 0 || durationMillis <= 0 {
		return 0
	}
	percent := int((outTimeMicros / 1000) * 100 / durationMillis)
	if percent > maxProgressPercent {
		return maxProgressPercent
	}
	if percent < 0 {
		return 0
	}
	return percent
}

// Transcode creates one browser-compatible MP4 rendition. It never upscales and caps
// landscape video at 1920x1080, portrait video at 1080x1920, and square video at 1080x1080.
func Transcode(ctx context.Context, inputPath, outputPath string, durationMillis int64, onProgress ProgressFunc) error {
	if inputPath == "" || outputPath == "" || durationMillis <= 0 {
		return fmt.Errorf("%w: invalid transcode arguments", ErrProcessing)
	}

	scaleFilter := "scale=w='if(gt(iw,ih),min(iw,1920),min(iw,1080))':h='if(gt(iw,ih),min(ih,1080),min(ih,1920))':force_original_aspect_ratio=decrease,scale=trunc(iw/2)*2:trunc(ih/2)*2"
	cmd := exec.CommandContext(
		ctx,
		"ffmpeg",
		"-y",
		"-v", "error",
		"-i", inputPath,
		"-map", "0:v:0",
		"-map", "0:a:0?",
		"-vf", scaleFilter,
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-crf", "23",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-b:a", "128k",
		"-movflags", "+faststart",
		"-progress", "pipe:1",
		"-nostats",
		outputPath,
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("%w: open ffmpeg progress pipe: %v", ErrProcessing, err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%w: start ffmpeg: %v", ErrProcessing, err)
	}

	scanner := bufio.NewScanner(stdout)
	lastProgress := -1
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "out_time_us=") {
			continue
		}
		value, parseErr := strconv.ParseInt(strings.TrimPrefix(line, "out_time_us="), 10, 64)
		if parseErr != nil {
			continue
		}
		progress := ProgressPercent(value, durationMillis)
		if onProgress != nil && progress != lastProgress {
			onProgress(progress)
			lastProgress = progress
		}
	}
	if err := scanner.Err(); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return fmt.Errorf("%w: read ffmpeg progress: %v", ErrProcessing, err)
	}
	if err := cmd.Wait(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if len(message) > 500 {
			message = message[len(message)-500:]
		}
		return fmt.Errorf("%w: ffmpeg exited: %v: %s", ErrProcessing, err, message)
	}
	return nil
}

// GenerateDefaultCover extracts one frame near the beginning of the video.
func GenerateDefaultCover(ctx context.Context, inputPath, outputDir string, durationMillis int64) (string, error) {
	if inputPath == "" || outputDir == "" || durationMillis <= 0 {
		return "", fmt.Errorf("%w: invalid cover arguments", ErrProcessing)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", fmt.Errorf("%w: create cover directory: %v", ErrProcessing, err)
	}

	seekMillis := int64(1000)
	if durationMillis < 2000 {
		seekMillis = durationMillis / 2
	}
	outputPath := filepath.Join(outputDir, "cover.jpg")
	cmd := exec.CommandContext(
		ctx,
		"ffmpeg",
		"-y",
		"-v", "error",
		"-ss", strconv.FormatFloat(float64(seekMillis)/1000, 'f', 3, 64),
		"-i", inputPath,
		"-frames:v", "1",
		"-q:v", "2",
		outputPath,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("%w: generate default cover: %v: %s", ErrProcessing, err, strings.TrimSpace(string(output)))
	}
	return outputPath, nil
}
