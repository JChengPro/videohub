package media

import (
	"context"
	"fmt"
	"os/exec"
)

// Probe invokes ffprobe and returns the normalized media information used by VideoHub.
func Probe(ctx context.Context, inputPath string) (MediaInfo, error) {
	if inputPath == "" {
		return MediaInfo{}, fmt.Errorf("%w: input path is empty", ErrInvalidMedia)
	}

	cmd := exec.CommandContext(
		ctx,
		"ffprobe",
		"-v", "error",
		"-print_format", "json",
		"-show_entries", "stream=codec_name,codec_type,width,height:stream_side_data=rotation:format=format_name,duration",
		inputPath,
	)
	output, err := cmd.Output()
	if err != nil {
		return MediaInfo{}, fmt.Errorf("%w: ffprobe failed: %v", ErrInvalidMedia, err)
	}
	return parseProbeOutput(output)
}
