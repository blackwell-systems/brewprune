package output

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mattn/go-isatty"
)

// writerIsTTY returns true if the given writer exposes an Fd() method
// (e.g. *os.File) and that fd is a terminal. Falls back to false for
// plain io.Writer values such as *bytes.Buffer.
func writerIsTTY(w io.Writer) bool {
	type fder interface {
		Fd() uintptr
	}
	if f, ok := w.(fder); ok {
		return isatty.IsTerminal(f.Fd())
	}
	return false
}

// ProgressBar displays a progress bar with percentage and description.
// Example: [=========>          ] 45% Installing packages...
type ProgressBar struct {
	total       int
	current     int
	description string
	width       int
	mu          sync.Mutex
	writer      io.Writer
}

// NewProgress creates a new progress bar.
func NewProgress(total int, description string) *ProgressBar {
	return &ProgressBar{
		total:       total,
		current:     0,
		description: description,
		width:       40, // default width in characters
		writer:      os.Stdout,
	}
}

// SetWidth sets the width of the progress bar in characters.
func (p *ProgressBar) SetWidth(width int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.width = width
}

// SetWriter sets the output writer (useful for testing).
func (p *ProgressBar) SetWriter(w io.Writer) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.writer = w
}

// Increment increments the progress by 1 and redraws the bar.
func (p *ProgressBar) Increment() {
	p.IncrementBy(1)
}

// IncrementBy increments the progress by n and redraws the bar.
func (p *ProgressBar) IncrementBy(n int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.current += n
	if p.current > p.total {
		p.current = p.total
	}

	p.render()
}

// SetCurrent sets the current progress value and redraws the bar.
func (p *ProgressBar) SetCurrent(current int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.current = current
	if p.current > p.total {
		p.current = p.total
	}

	p.render()
}

// Finish completes the progress bar and moves to a new line.
func (p *ProgressBar) Finish() {
	p.mu.Lock()
	defer p.mu.Unlock()

	alreadyDone := p.current == p.total
	p.current = p.total

	if writerIsTTY(p.writer) {
		// TTY: render() uses \r (no newline), so always re-render and then newline.
		p.render()
		fmt.Fprintln(p.writer)
	} else {
		// Non-TTY: render() emits a newline only when current==total.
		// If we were already at total (e.g. last Increment already emitted),
		// skip the re-render to avoid a duplicate 100% line.
		if !alreadyDone {
			p.render()
		}
	}
}

// render draws the progress bar (must be called with lock held).
func (p *ProgressBar) render() {
	percentage := 0
	if p.total > 0 {
		percentage = (p.current * 100) / p.total
	}

	// Calculate filled portion
	filled := 0
	if p.total > 0 {
		filled = (p.current * p.width) / p.total
	}

	// Build the bar
	bar := strings.Builder{}
	bar.WriteString("[")

	for i := 0; i < p.width; i++ {
		if i < filled-1 {
			bar.WriteString("=")
		} else if i == filled-1 {
			bar.WriteString(">")
		} else {
			bar.WriteString(" ")
		}
	}

	bar.WriteString("]")

	if writerIsTTY(p.writer) {
		// TTY: overwrite the current line using carriage return
		fmt.Fprintf(p.writer, "\r%s %3d%% %s", bar.String(), percentage, p.description)
	} else {
		// Non-TTY: only emit output on completion to avoid duplicate lines
		if p.current == p.total {
			fmt.Fprintf(p.writer, "%s %3d%% %s\n", bar.String(), percentage, p.description)
		}
	}
}

// Spinner displays an animated spinner with a message.
// Example: |  Analyzing packages...
type Spinner struct {
	message    string
	running    bool
	chars      []string
	mu         sync.Mutex
	writer     io.Writer
	ticker     *time.Ticker
	done       chan struct{}
	timeout    time.Duration
	startTime  time.Time
	showTiming bool
}

// NewSpinner creates a new spinner with a message.
// If stdout is not a TTY, the animation goroutine is skipped and the
// message is printed once so that log output is not cluttered.
//
// Use WithTimeout() before the spinner starts to add time estimates:
//   spinner := output.NewSpinner("Working...")
//   spinner.WithTimeout(30 * time.Second)
func NewSpinner(message string) *Spinner {
	s := &Spinner{
		message:    message,
		running:    false,
		chars:      []string{"|", "/", "-", "\\"},
		writer:     os.Stdout,
		done:       make(chan struct{}),
		showTiming: false,
	}
	// Note: Don't call Start() here - let WithTimeout be called first if needed
	return s
}

// WithTimeout configures the spinner to show elapsed time and optionally
// a timeout duration. If timeout is > 0, displays remaining time format
// "message (Xs remaining)"; otherwise displays elapsed time format
// "message (Xs elapsed)".
//
// This method must be called before Start(). It returns the spinner for chaining.
func (s *Spinner) WithTimeout(timeout time.Duration) *Spinner {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.timeout = timeout
	s.showTiming = true
	return s
}

// SetWriter sets the output writer (useful for testing).
func (s *Spinner) SetWriter(w io.Writer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writer = w
}

// Start begins the spinner animation.
// On a non-TTY writer the animation goroutine is not started; the message
// is printed once instead so that non-interactive output stays clean.
func (s *Spinner) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return
	}

	s.running = true
	s.startTime = time.Now()

	if !writerIsTTY(s.writer) {
		// Non-TTY: print message once and return; no goroutine needed.
		fmt.Fprintf(s.writer, "%s...\n", s.message)
		return
	}

	s.ticker = time.NewTicker(100 * time.Millisecond)

	go func() {
		idx := 0
		for {
			select {
			case <-s.ticker.C:
				s.mu.Lock()
				if !s.running {
					s.mu.Unlock()
					return
				}
				msg := s.formatMessage()
				fmt.Fprintf(s.writer, "\r%s  %s", s.chars[idx], msg)
				idx = (idx + 1) % len(s.chars)
				s.mu.Unlock()

			case <-s.done:
				return
			}
		}
	}()
}

// formatMessage returns the spinner message with optional timing information.
// Must be called with lock held.
func (s *Spinner) formatMessage() string {
	if !s.showTiming {
		return s.message
	}

	elapsed := time.Since(s.startTime)
	if s.timeout > 0 {
		// Show remaining time format: "message (12s remaining)"
		remaining := s.timeout - elapsed
		if remaining < 0 {
			remaining = 0
		}
		return fmt.Sprintf("%s (%ds remaining)", s.message, int(remaining.Seconds()))
	}

	// Show elapsed time format: "message (5s elapsed)"
	return fmt.Sprintf("%s (%ds elapsed)", s.message, int(elapsed.Seconds()))
}

// Stop stops the spinner animation and clears the line.
func (s *Spinner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.running = false
	if s.ticker != nil {
		s.ticker.Stop()
	}
	close(s.done)

	// Clear the line only on a TTY — on non-TTY the \r does not overwrite.
	if writerIsTTY(s.writer) {
		fmt.Fprintf(s.writer, "\r%s\r", strings.Repeat(" ", len(s.message)+4))
	}
}

// UpdateMessage updates the spinner message while it's running.
func (s *Spinner) UpdateMessage(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.message = message
}

// StopWithMessage stops the spinner and displays a final message.
func (s *Spinner) StopWithMessage(message string) {
	s.Stop()
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Fprintln(s.writer, message)
}
