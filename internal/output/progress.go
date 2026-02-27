package output

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

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

	p.current = p.total
	p.render()
	fmt.Fprintln(p.writer) // Move to next line
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

	// Clear line and print progress
	fmt.Fprintf(p.writer, "\r%s %3d%% %s", bar.String(), percentage, p.description)
}

// Spinner displays an animated spinner with a message.
// Example: |  Analyzing packages...
type Spinner struct {
	message string
	running bool
	chars   []string
	mu      sync.Mutex
	writer  io.Writer
	ticker  *time.Ticker
	done    chan struct{}
}

// NewSpinner creates a new spinner with a message.
func NewSpinner(message string) *Spinner {
	s := &Spinner{
		message: message,
		running: false,
		chars:   []string{"|", "/", "-", "\\"},
		writer:  os.Stdout,
		done:    make(chan struct{}),
	}
	s.Start()
	return s
}

// SetWriter sets the output writer (useful for testing).
func (s *Spinner) SetWriter(w io.Writer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writer = w
}

// Start begins the spinner animation.
func (s *Spinner) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return
	}

	s.running = true
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
				fmt.Fprintf(s.writer, "\r%s  %s", s.chars[idx], s.message)
				idx = (idx + 1) % len(s.chars)
				s.mu.Unlock()

			case <-s.done:
				return
			}
		}
	}()
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

	// Clear the line
	fmt.Fprintf(s.writer, "\r%s\r", strings.Repeat(" ", len(s.message)+4))
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
