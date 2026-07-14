// Package media wraps the local media tools (FFmpeg, Whisper) used by the
// ingest stage of the Latent Frame pipeline. Ported and trimmed from the
// author's earlier `videra` project.
package media

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// FFmpegRunner extracts audio and frames from a source video.
type FFmpegRunner interface {
	ExtractAudio(ctx context.Context, videoPath, outputPath string) error
	ExtractKeyframes(ctx context.Context, videoPath, outputDir string, intervalSec int) error
	ExtractFrames(ctx context.Context, videoPath, outputDir string, fps float64, maxWidth int) error
	SceneTimestamps(ctx context.Context, videoPath string, threshold float64) ([]float64, error)
}

// ExecFFmpeg runs the system `ffmpeg` binary.
type ExecFFmpeg struct{}

// ExtractAudio writes the video's audio track to outputPath as MP3.
func (ExecFFmpeg) ExtractAudio(ctx context.Context, videoPath, outputPath string) error {
	cmd := exec.CommandContext(
		ctx,
		"ffmpeg",
		"-y",
		"-i", videoPath,
		"-vn",
		"-acodec", "mp3",
		outputPath,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg extract audio failed: %w: %s", err, string(output))
	}
	return nil
}

// ExtractKeyframes samples one frame every intervalSec seconds into outputDir as
// frame-NNNNN.jpg. This is deliberately naive interval sampling — smart, quality-
// aware hero-frame selection (sharpness/composition per room) is stage-1 work that
// belongs in the pipeline package, not here.
func (ExecFFmpeg) ExtractKeyframes(ctx context.Context, videoPath, outputDir string, intervalSec int) error {
	if intervalSec <= 0 {
		intervalSec = 5
	}

	outputPattern := fmt.Sprintf("%s/frame-%%05d.jpg", outputDir)
	filter := fmt.Sprintf("fps=1/%d", intervalSec)

	cmd := exec.CommandContext(
		ctx,
		"ffmpeg",
		"-y",
		"-i", videoPath,
		"-vf", filter,
		outputPattern,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg extract keyframes failed: %w: %s", err, string(output))
	}

	return nil
}

// ExtractFrames samples the video at fps frames/second into outputDir as
// frame-NNNNN.jpg, downscaled to at most maxWidth (aspect preserved). Frame N
// (1-based) sits at roughly (N-1)/fps seconds. Dense candidates for hero selection.
func (ExecFFmpeg) ExtractFrames(ctx context.Context, videoPath, outputDir string, fps float64, maxWidth int) error {
	if fps <= 0 {
		fps = 2
	}
	if maxWidth <= 0 {
		maxWidth = 1280
	}
	filter := fmt.Sprintf("fps=%g,scale='min(%d,iw)':-2", fps, maxWidth)
	pattern := fmt.Sprintf("%s/frame-%%05d.jpg", outputDir)
	cmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-i", videoPath, "-vf", filter, "-qscale:v", "3", pattern)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg extract frames failed: %w: %s", err, string(output))
	}
	return nil
}

// SceneTimestamps returns the times (seconds) where ffmpeg's scene score exceeds
// threshold (0..1) — i.e. shot boundaries. Empty for continuous handheld footage,
// which is expected; callers fall back to time-binning.
func (ExecFFmpeg) SceneTimestamps(ctx context.Context, videoPath string, threshold float64) ([]float64, error) {
	if threshold <= 0 {
		threshold = 0.4
	}
	// metadata=print with no file= writes the per-cut scene metadata to ffmpeg's log
	// (captured here via CombinedOutput) rather than a temp file. This avoids embedding
	// the temp path — unescaped — in the filtergraph, which breaks on some platforms
	// (e.g. Windows paths with ':' and '\'). Verified to yield identical pts_time output.
	filter := fmt.Sprintf("select='gt(scene,%.3f)',metadata=print", threshold)
	cmd := exec.CommandContext(ctx, "ffmpeg", "-hide_banner", "-loglevel", "info", "-i", videoPath, "-vf", filter, "-an", "-f", "null", "-")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg scene detect failed: %w: %s", err, string(output))
	}
	var times []float64
	for _, line := range strings.Split(string(output), "\n") {
		i := strings.Index(line, "pts_time:")
		if i < 0 {
			continue
		}
		fields := strings.Fields(line[i+len("pts_time:"):])
		if len(fields) == 0 {
			continue
		}
		if t, err := strconv.ParseFloat(fields[0], 64); err == nil {
			times = append(times, t)
		}
	}
	return times, nil
}

var _ FFmpegRunner = (*ExecFFmpeg)(nil)
