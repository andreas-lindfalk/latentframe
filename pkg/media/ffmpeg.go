// Package media wraps the local media tools (FFmpeg, Whisper) used by the
// ingest stage of the Latent Frame pipeline. Ported and trimmed from the
// author's earlier `videra` project.
package media

import (
	"context"
	"fmt"
	"os/exec"
)

// FFmpegRunner extracts audio and keyframes from a source video.
type FFmpegRunner interface {
	ExtractAudio(ctx context.Context, videoPath, outputPath string) error
	ExtractKeyframes(ctx context.Context, videoPath, outputDir string, intervalSec int) error
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

var _ FFmpegRunner = (*ExecFFmpeg)(nil)
