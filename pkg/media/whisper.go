package media

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Transcriber turns an audio file into ordered, time-stamped narration segments.
type Transcriber interface {
	Transcribe(ctx context.Context, audioPath string) ([]Segment, error)
}

// WhisperCLITranscriber shells out to the OpenAI Whisper CLI (`whisper`, or
// `python3 -m whisper` as a fallback). Model is overridable via the
// LATENTFRAME_WHISPER_MODEL env var; language is an optional hint ("es", "en", ...).
type WhisperCLITranscriber struct {
	model    string
	language string
}

// NewWhisperCLITranscriber builds a transcriber. Pass an empty language for
// Whisper's auto-detection.
func NewWhisperCLITranscriber(language string) *WhisperCLITranscriber {
	model := strings.TrimSpace(os.Getenv("LATENTFRAME_WHISPER_MODEL"))
	if model == "" {
		model = "base"
	}
	return &WhisperCLITranscriber{model: model, language: strings.TrimSpace(language)}
}

// Transcribe extracts timed segments from audioPath.
func (t *WhisperCLITranscriber) Transcribe(ctx context.Context, audioPath string) ([]Segment, error) {
	if strings.TrimSpace(audioPath) == "" {
		return nil, fmt.Errorf("audio path is required")
	}
	if _, err := os.Stat(audioPath); err != nil {
		return nil, fmt.Errorf("audio path not found: %w", err)
	}

	outputDir, err := os.MkdirTemp("", "latentframe-whisper-*")
	if err != nil {
		return nil, fmt.Errorf("create whisper output dir: %w", err)
	}
	defer os.RemoveAll(outputDir)

	if err := runWhisperCLI(ctx, audioPath, outputDir, t.model, t.language); err != nil {
		return nil, err
	}

	base := strings.TrimSuffix(filepath.Base(audioPath), filepath.Ext(audioPath))
	srtPath := filepath.Join(outputDir, base+".srt")
	payload, err := os.ReadFile(srtPath)
	if err != nil {
		return nil, fmt.Errorf("read whisper transcript output: %w", err)
	}

	return parseSRT(string(payload), 5)
}

func runWhisperCLI(ctx context.Context, audioPath, outputDir, model, language string) error {
	args := []string{
		audioPath,
		"--task", "transcribe",
		"--output_format", "srt",
		"--output_dir", outputDir,
		"--model", model,
		"--fp16", "False",
	}
	if language != "" {
		args = append(args, "--language", language)
	}

	var lastErr error
	if _, err := exec.LookPath("whisper"); err == nil {
		cmd := exec.CommandContext(ctx, "whisper", args...)
		if output, runErr := cmd.CombinedOutput(); runErr != nil {
			lastErr = fmt.Errorf("whisper transcription failed: %w: %s", runErr, strings.TrimSpace(string(output)))
		} else {
			return nil
		}
	}

	if _, err := exec.LookPath("python3"); err == nil {
		pythonArgs := append([]string{"-m", "whisper"}, args...)
		cmd := exec.CommandContext(ctx, "python3", pythonArgs...)
		if output, runErr := cmd.CombinedOutput(); runErr != nil {
			lastErr = fmt.Errorf("python whisper transcription failed: %w: %s", runErr, strings.TrimSpace(string(output)))
		} else {
			return nil
		}
	}

	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("whisper CLI not found; install `whisper` (or `python3 -m whisper`)")
}

var _ Transcriber = (*WhisperCLITranscriber)(nil)
