package downloader

import (
	"fmt"
	"io"
	"sync"

	"github.com/ducminhgd/manga-chef/pkg/sources"
)

// NewTerminalProgressReporter returns a progress reporter that writes simple
// progress lines to the provided writer.
func NewTerminalProgressReporter(out io.Writer) ProgressReporter {
	return &terminalProgressReporter{out: out}
}

type terminalProgressReporter struct {
	out     io.Writer
	started map[string]bool
	mu      sync.Mutex
}

func (t *terminalProgressReporter) OnStart(chapter sources.Chapter, total int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.started == nil {
		t.started = map[string]bool{}
	}
	key := fmt.Sprintf("%.2f-%s", chapter.Number, chapter.Title)
	if t.started[key] {
		return
	}
	t.started[key] = true
	fmt.Fprintf(t.out, "Downloading chapter %.2f - %q: 0/%d\n", chapter.Number, chapter.Title, total)
}

func (t *terminalProgressReporter) OnProgress(chapter sources.Chapter, done int, total int) {
	fmt.Fprintf(t.out, "Chapter %.2f: %d/%d pages downloaded\n", chapter.Number, done, total)
}

func (t *terminalProgressReporter) OnDone(chapter sources.Chapter) {
	fmt.Fprintf(t.out, "Completed chapter %.2f\n", chapter.Number)
}

func (t *terminalProgressReporter) OnError(err error) {
	fmt.Fprintf(t.out, "[WARNING] %v\n", err)
}
