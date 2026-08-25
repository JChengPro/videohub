package media

//把ffprobe输出的一大段JSON，整理成VideoHub能直接使用的视频信息。

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

var ErrInvalidMedia = errors.New("invalid video media")

type MediaInfo struct {
	VideoCodec     string
	AudioCodec     string
	FormatName     string
	Width          int
	Height         int
	DurationMillis int64
}

type probeSideData struct {
	Rotation int `json:"rotation"`
}

type probeStream struct {
	CodecName    string          `json:"codec_name"`
	CodecType    string          `json:"codec_type"`
	Width        int             `json:"width"`
	Height       int             `json:"height"`
	SideDataList []probeSideData `json:"side_data_list"`
}

type probeFormat struct {
	FormatName string `json:"format_name"`
	Duration   string `json:"duration"`
}

type probeOutput struct {
	Streams []probeStream `json:"streams"`
	Format  probeFormat   `json:"format"`
}

func rotationSwapsDimensions(rotation int) bool {
	normalized := ((rotation % 360) + 360) % 360
	return normalized == 90 || normalized == 270
}

func parseProbeOutput(data []byte) (MediaInfo, error) {
	var output probeOutput
	if err := json.Unmarshal(data, &output); err != nil {
		return MediaInfo{}, fmt.Errorf("%w: decode ffprobe JSON: %v", ErrInvalidMedia, err)
	}

	durationSeconds, err := strconv.ParseFloat(
		strings.TrimSpace(output.Format.Duration),
		64,
	)
	if err != nil || durationSeconds <= 0 {
		return MediaInfo{}, fmt.Errorf(
			"%w: invalid duration %q",
			ErrInvalidMedia,
			output.Format.Duration,
		)
	}

	info := MediaInfo{
		FormatName:     output.Format.FormatName,
		DurationMillis: int64(math.Round(durationSeconds * 1000)),
	}

	for _, stream := range output.Streams {
		switch stream.CodecType {
		case "video":
			if info.VideoCodec != "" {
				continue
			}
			width := stream.Width
			height := stream.Height
			for _, sideData := range stream.SideDataList {
				if rotationSwapsDimensions(sideData.Rotation) {
					width, height = height, width
					break
				}
			}
			info.VideoCodec = stream.CodecName
			info.Width = width
			info.Height = height
		case "audio":
			if info.AudioCodec == "" {
				info.AudioCodec = stream.CodecName
			}
		}
	}

	if info.VideoCodec == "" {
		return MediaInfo{}, fmt.Errorf("%w: video stream not found", ErrInvalidMedia)
	}
	if info.Width <= 0 || info.Height <= 0 {
		return MediaInfo{}, fmt.Errorf(
			"%w: invalid video dimensions %dx%d",
			ErrInvalidMedia,
			info.Width,
			info.Height,
		)
	}

	return info, nil
}
