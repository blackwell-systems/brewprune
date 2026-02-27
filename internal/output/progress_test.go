package output

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestProgressBar_Basic(t *testing.T) {
	buf := &bytes.Buffer{}
	p := NewProgress(100, "Testing")
	p.SetWriter(buf)

	// Initial state
	p.render()
	output := buf.String()

	if !strings.Contains(output, "[") || !strings.Contains(output, "]") {
		t.Errorf("Progress bar should contain brackets, got: %q", output)
	}
	if !strings.Contains(output, "0%") {
		t.Errorf("Initial progress should be 0%%, got: %q", output)
	}
	if !strings.Contains(output, "Testing") {
		t.Errorf("Progress bar should contain description, got: %q", output)
	}
}

func TestProgressBar_Increment(t *testing.T) {
	buf := &bytes.Buffer{}
	p := NewProgress(10, "Processing")
	p.SetWriter(buf)

	// Increment by 1
	p.Increment()
	output := buf.String()

	if !strings.Contains(output, "10%") {
		t.Errorf("After 1/10 increment, should show 10%%, got: %q", output)
	}

	// Increment to 50%
	buf.Reset()
	p.SetCurrent(5)
	output = buf.String()

	if !strings.Contains(output, "50%") {
		t.Errorf("After 5/10 current, should show 50%%, got: %q", output)
	}
}

func TestProgressBar_IncrementBy(t *testing.T) {
	buf := &bytes.Buffer{}
	p := NewProgress(100, "Loading")
	p.SetWriter(buf)

	// Increment by 25
	p.IncrementBy(25)
	output := buf.String()

	if !strings.Contains(output, "25%") {
		t.Errorf("After incrementing by 25, should show 25%%, got: %q", output)
	}

	// Increment by another 25
	buf.Reset()
	p.IncrementBy(25)
	output = buf.String()

	if !strings.Contains(output, "50%") {
		t.Errorf("After incrementing by 50 total, should show 50%%, got: %q", output)
	}
}

func TestProgressBar_SetCurrent(t *testing.T) {
	buf := &bytes.Buffer{}
	p := NewProgress(100, "Downloading")
	p.SetWriter(buf)

	tests := []struct {
		current int
		percent string
	}{
		{0, "0%"},
		{25, "25%"},
		{50, "50%"},
		{75, "75%"},
		{100, "100%"},
	}

	for _, tt := range tests {
		t.Run(tt.percent, func(t *testing.T) {
			buf.Reset()
			p.SetCurrent(tt.current)
			output := buf.String()

			if !strings.Contains(output, tt.percent) {
				t.Errorf("SetCurrent(%d) should show %s, got: %q", tt.current, tt.percent, output)
			}
		})
	}
}

func TestProgressBar_Finish(t *testing.T) {
	buf := &bytes.Buffer{}
	p := NewProgress(100, "Complete")
	p.SetWriter(buf)

	p.SetCurrent(75)
	buf.Reset()
	p.Finish()
	output := buf.String()

	if !strings.Contains(output, "100%") {
		t.Errorf("Finish() should show 100%%, got: %q", output)
	}
	if !strings.HasSuffix(strings.TrimSpace(output), "Complete") {
		t.Errorf("Finish() should end with description, got: %q", output)
	}
}

func TestProgressBar_OverLimit(t *testing.T) {
	buf := &bytes.Buffer{}
	p := NewProgress(10, "Test")
	p.SetWriter(buf)

	// Try to increment beyond total
	p.IncrementBy(15)
	output := buf.String()

	if !strings.Contains(output, "100%") {
		t.Errorf("Progress should cap at 100%%, got: %q", output)
	}

	// Try to set beyond total
	buf.Reset()
	p.SetCurrent(20)
	output = buf.String()

	if !strings.Contains(output, "100%") {
		t.Errorf("Progress should cap at 100%%, got: %q", output)
	}
}

func TestProgressBar_ZeroTotal(t *testing.T) {
	buf := &bytes.Buffer{}
	p := NewProgress(0, "Empty")
	p.SetWriter(buf)

	// Should not panic with zero total
	p.Increment()
	output := buf.String()

	if !strings.Contains(output, "[") || !strings.Contains(output, "]") {
		t.Errorf("Progress bar with zero total should still render, got: %q", output)
	}
}

func TestProgressBar_Width(t *testing.T) {
	buf := &bytes.Buffer{}
	p := NewProgress(100, "Test")
	p.SetWriter(buf)
	p.SetWidth(20)

	p.SetCurrent(50)
	output := buf.String()

	// Count the characters between [ and ]
	start := strings.Index(output, "[")
	end := strings.Index(output, "]")

	if start == -1 || end == -1 {
		t.Fatalf("Could not find brackets in output: %q", output)
	}

	barContent := output[start+1 : end]
	if len(barContent) != 20 {
		t.Errorf("Progress bar width should be 20, got %d: %q", len(barContent), barContent)
	}
}

func TestProgressBar_VisualRender(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping visual test in short mode")
	}

	buf := &bytes.Buffer{}
	p := NewProgress(100, "Installing packages")
	p.SetWriter(buf)

	// Test various progress levels
	progressLevels := []int{0, 10, 25, 45, 50, 75, 90, 100}

	for _, level := range progressLevels {
		buf.Reset()
		p.SetCurrent(level)
		t.Logf("Progress %d%%:\n%s", level, buf.String())
	}
}

func TestSpinner_Basic(t *testing.T) {
	buf := &bytes.Buffer{}
	s := NewSpinner("Loading")
	s.SetWriter(buf)

	// Give it a moment to start
	time.Sleep(150 * time.Millisecond)

	s.Stop()
	output := buf.String()

	// Should have rendered at least once
	if len(output) == 0 {
		t.Error("Spinner should produce output")
	}
}

func TestSpinner_StartStop(t *testing.T) {
	buf := &bytes.Buffer{}
	s := &Spinner{
		message: "Test",
		chars:   []string{"|", "/", "-", "\\"},
		writer:  buf,
		done:    make(chan struct{}),
	}

	// Start spinner
	s.Start()

	if !s.running {
		t.Error("Spinner should be running after Start()")
	}

	// Wait for at least one tick
	time.Sleep(150 * time.Millisecond)

	// Stop spinner
	s.Stop()

	if s.running {
		t.Error("Spinner should not be running after Stop()")
	}
}

func TestSpinner_MultipleStops(t *testing.T) {
	buf := &bytes.Buffer{}
	s := NewSpinner("Test")
	s.SetWriter(buf)

	// Wait for it to start
	time.Sleep(50 * time.Millisecond)

	// Multiple stops should not panic
	s.Stop()
	s.Stop()
	s.Stop()
}

func TestSpinner_UpdateMessage(t *testing.T) {
	buf := &bytes.Buffer{}
	s := NewSpinner("Initial")
	s.SetWriter(buf)

	// Wait for initial render
	time.Sleep(50 * time.Millisecond)

	// Update message
	s.UpdateMessage("Updated")

	// Wait for updated render
	time.Sleep(150 * time.Millisecond)

	s.Stop()

	output := buf.String()
	if !strings.Contains(output, "Updated") {
		t.Errorf("Spinner should contain updated message, got: %q", output)
	}
}

func TestSpinner_StopWithMessage(t *testing.T) {
	buf := &bytes.Buffer{}
	s := NewSpinner("Working")
	s.SetWriter(buf)

	// Wait for spinner to run
	time.Sleep(150 * time.Millisecond)

	// Stop with a final message
	s.StopWithMessage("Done!")

	output := buf.String()
	if !strings.Contains(output, "Done!") {
		t.Errorf("Spinner should contain final message, got: %q", output)
	}
}

func TestSpinner_Animation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping visual test in short mode")
	}

	buf := &bytes.Buffer{}
	s := NewSpinner("Analyzing packages")
	s.SetWriter(buf)

	// Let it spin for a bit
	time.Sleep(500 * time.Millisecond)

	s.Stop()

	output := buf.String()
	t.Logf("Spinner output:\n%s", output)

	// Should have cycled through multiple characters
	hasBar := strings.Contains(output, "|")
	hasSlash := strings.Contains(output, "/")
	hasDash := strings.Contains(output, "-")
	hasBackslash := strings.Contains(output, "\\")

	if !hasBar && !hasSlash && !hasDash && !hasBackslash {
		t.Error("Spinner should have rendered at least one animation character")
	}
}

// TestProgressBar_Concurrent tests thread safety
func TestProgressBar_Concurrent(t *testing.T) {
	buf := &bytes.Buffer{}
	p := NewProgress(1000, "Concurrent test")
	p.SetWriter(buf)

	// Launch multiple goroutines incrementing concurrently
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				p.Increment()
			}
			done <- struct{}{}
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should have reached 100%
	buf.Reset()
	p.render()
	output := buf.String()

	if !strings.Contains(output, "100%") {
		t.Errorf("After concurrent increments, should be at 100%%, got: %q", output)
	}
}

// TestSpinner_Concurrent tests spinner thread safety
func TestSpinner_Concurrent(t *testing.T) {
	buf := &bytes.Buffer{}
	s := NewSpinner("Concurrent spinner")
	s.SetWriter(buf)

	// Update message from multiple goroutines
	done := make(chan struct{})
	for i := 0; i < 5; i++ {
		go func(n int) {
			for j := 0; j < 10; j++ {
				s.UpdateMessage("Message from goroutine")
				time.Sleep(10 * time.Millisecond)
			}
			done <- struct{}{}
		}(i)
	}

	// Wait for all updates
	for i := 0; i < 5; i++ {
		<-done
	}

	s.Stop()
	// Should not panic or race
}

// Benchmark tests
func BenchmarkProgressBar_Increment(b *testing.B) {
	buf := &bytes.Buffer{}
	p := NewProgress(b.N, "Benchmark")
	p.SetWriter(buf)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Increment()
	}
}

func BenchmarkProgressBar_Render(b *testing.B) {
	buf := &bytes.Buffer{}
	p := NewProgress(100, "Benchmark")
	p.SetWriter(buf)
	p.SetCurrent(50)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.render()
	}
}

func BenchmarkFormatSize(b *testing.B) {
	sizes := []int64{
		512,
		1024 * 1024,
		1024 * 1024 * 1024,
		10 * 1024 * 1024 * 1024,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		formatSize(sizes[i%len(sizes)])
	}
}

func BenchmarkFormatRelativeTime(b *testing.B) {
	times := []time.Time{
		time.Now().Add(-30 * time.Second),
		time.Now().Add(-5 * time.Minute),
		time.Now().Add(-2 * time.Hour),
		time.Now().Add(-3 * 24 * time.Hour),
		time.Now().Add(-30 * 24 * time.Hour),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		formatRelativeTime(times[i%len(times)])
	}
}
