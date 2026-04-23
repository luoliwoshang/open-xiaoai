package speaker

import (
	"fmt"
	"strings"
	"time"

	"github.com/idootop/open-xiaoai/examples/go-instruction-server/internal/server"
)

type ShellRunner interface {
	RunShell(script string, timeout time.Duration) (server.CommandResult, error)
}

type Speaker struct{}

func New() *Speaker {
	return &Speaker{}
}

func (s *Speaker) PlayText(runner ShellRunner, text string, timeout time.Duration) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("empty text")
	}

	result, err := runner.RunShell("/usr/sbin/tts_play.sh "+shellQuote(text), timeout)
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("exit_code=%d stderr=%s", result.ExitCode, strings.TrimSpace(result.Stderr))
	}

	return nil
}

func (s *Speaker) PlayTextStream(runner ShellRunner, chunks []string, timeout time.Duration, gap time.Duration) error {
	for i, chunk := range chunks {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}

		if err := s.PlayText(runner, chunk, timeout); err != nil {
			return err
		}

		if gap > 0 && i < len(chunks)-1 {
			time.Sleep(gap)
		}
	}

	return nil
}

func shellQuote(text string) string {
	return "'" + strings.ReplaceAll(text, "'", `'"'"'`) + "'"
}
