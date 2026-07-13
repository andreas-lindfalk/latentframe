package media

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var timeRangePattern = regexp.MustCompile(`(\d{2}:\d{2}:\d{2}[\.,]\d{3})\s*-->\s*(\d{2}:\d{2}:\d{2}[\.,]\d{3})`)

// parseSRT parses SRT/VTT timed captions into ordered Segments. fallbackIntervalSec
// is used to synthesize an end time when a caption block has a non-positive range.
func parseSRT(raw string, fallbackIntervalSec int) ([]Segment, error) {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	blocks := strings.Split(normalized, "\n\n")

	segments := make([]Segment, 0, len(blocks))
	for _, block := range blocks {
		lines := strings.Split(block, "\n")
		clean := make([]string, 0, len(lines))
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			if strings.EqualFold(trimmed, "WEBVTT") {
				continue
			}
			clean = append(clean, trimmed)
		}
		if len(clean) == 0 {
			continue
		}

		timeLineIndex := -1
		for idx, line := range clean {
			if strings.Contains(line, "-->") {
				timeLineIndex = idx
				break
			}
		}
		if timeLineIndex == -1 {
			continue
		}

		match := timeRangePattern.FindStringSubmatch(clean[timeLineIndex])
		if len(match) != 3 {
			continue
		}

		startMs, err := parseTimecodeToMs(match[1])
		if err != nil {
			continue
		}
		endMs, err := parseTimecodeToMs(match[2])
		if err != nil {
			continue
		}
		if endMs <= startMs {
			endMs = startMs + int64(fallbackIntervalSec*1000)
		}

		text := strings.TrimSpace(strings.Join(clean[timeLineIndex+1:], " "))
		if text == "" {
			continue
		}

		segments = append(segments, Segment{StartMs: startMs, EndMs: endMs, Text: text})
	}

	if len(segments) == 0 {
		return nil, fmt.Errorf("transcript contains no parseable timed captions")
	}

	return segments, nil
}

func parseTimecodeToMs(input string) (int64, error) {
	normalized := strings.ReplaceAll(strings.TrimSpace(input), ",", ".")
	parts := strings.Split(normalized, ":")
	if len(parts) != 3 {
		return 0, fmt.Errorf("invalid timecode: %s", input)
	}

	hours, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, err
	}
	minutes, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, err
	}
	secParts := strings.Split(parts[2], ".")
	if len(secParts) != 2 {
		return 0, fmt.Errorf("invalid timecode seconds: %s", input)
	}
	seconds, err := strconv.Atoi(secParts[0])
	if err != nil {
		return 0, err
	}
	millis, err := strconv.Atoi(secParts[1])
	if err != nil {
		return 0, err
	}
	if millis < 10 {
		millis *= 100
	} else if millis < 100 {
		millis *= 10
	}

	return int64(hours*3600*1000 + minutes*60*1000 + seconds*1000 + millis), nil
}
