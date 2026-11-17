package services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const (
	demoConvertMaxInputBytes  = 30 << 20
	demoConvertMaxOutputBytes = 40 << 20
	demoConvertMaxSeconds     = 15
)

var (
	ErrDemoConvertBusy        = errors.New("conversion is busy, try again shortly")
	ErrDemoConvertInput       = errors.New("choose an MP4, WebM or MOV clip under 30 MB")
	ErrDemoConvertOutput      = errors.New("the clip could not be converted")
	ErrDemoConvertUnavailable = errors.New("video conversion is not available on this server")
)

type DemoConvertService struct {
	ffmpegPath string
	slots      chan struct{}
}

func NewDemoConvertService(ffmpegPath string) *DemoConvertService {
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}
	return &DemoConvertService{ffmpegPath: ffmpegPath, slots: make(chan struct{}, 2)}
}

func (s *DemoConvertService) Available() bool {
	_, err := exec.LookPath(s.ffmpegPath)
	return err == nil
}

func (s *DemoConvertService) Convert(ctx context.Context, input []byte, extension string) ([]byte, error) {
	if !s.Available() {
		return nil, ErrDemoConvertUnavailable
	}
	if len(input) == 0 || len(input) > demoConvertMaxInputBytes {
		return nil, ErrDemoConvertInput
	}
	switch extension {
	case ".mp4", ".webm", ".mov":
	default:
		return nil, ErrDemoConvertInput
	}

	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	default:
		return nil, ErrDemoConvertBusy
	}

	dir, err := os.MkdirTemp("", "snappsy-demo-convert-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	inPath := filepath.Join(dir, "source"+extension)
	if err := os.WriteFile(inPath, input, 0o600); err != nil {
		return nil, err
	}
	outPath := filepath.Join(dir, "loop.mp4")

	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, s.ffmpegPath,
		"-hide_banner", "-loglevel", "error", "-y",
		"-i", inPath,
		"-t", fmt.Sprint(demoConvertMaxSeconds),
		"-an",
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-crf", "26",
		"-pix_fmt", "yuv420p",
		"-movflags", "+faststart",
		outPath,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrDemoConvertOutput, string(output))
	}

	info, err := os.Stat(outPath)
	if err != nil || info.Size() == 0 || info.Size() > demoConvertMaxOutputBytes {
		return nil, ErrDemoConvertOutput
	}
	return os.ReadFile(outPath)
}
