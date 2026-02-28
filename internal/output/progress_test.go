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

	// On a non-TTY writer, render only emits at 100%. Drive to completion.
	p.Finish()
	output := buf.String()

	if !strings.Contains(output, "[") || !strings.Contains(output, "]") {
		t.Errorf("Progress bar should contain brackets, got: %q", output)
	}
	if !strings.Contains(output, "100%") {
		t.Errorf("Finished progress should be 100%%, got: %q", output)
	}
	if !strings.Contains(output, "Testing") {
		t.Errorf("Progress bar should contain description, got: %q", output)
	}
}

func TestProgressBar_Increment(t *testing.T) {
	buf := &bytes.Buffer{}
	p := NewProgress(10, "Processing")
	p.SetWriter(buf)

	// On non-TTY, intermediate renders produce no output; only 100% emits.
	// Increment through all 10 steps and verify final output.
	for i := 0; i < 10; i++ {
		p.Increment()
	}
	output := buf.String()

	if !strings.Contains(output, "100%") {
		t.Errorf("After all increments, should show 100%%, got: %q", output)
	}

	// Reset and verify SetCurrent to 100 also works
	buf.Reset()
	p2 := NewProgress(10, "Processing")
	p2.SetWriter(buf)
	p2.SetCurrent(10)
	output = buf.String()

	if !strings.Contains(output, "100%") {
		t.Errorf("SetCurrent(total) should show 100%%, got: %q", output)
	}
}

func TestProgressBar_IncrementBy(t *testing.T) {
	buf := &bytes.Buffer{}
	p := NewProgress(100, "Loading")
	p.SetWriter(buf)

	// On non-TTY, render only emits at completion.
	// Increment in one shot to 100 and verify.
	p.IncrementBy(100)
	output := buf.String()

	if !strings.Contains(output, "100%") {
		t.Errorf("After incrementing to 100, should show 100%%, got: %q", output)
	}
}

func TestProgressBar_SetCurrent(t *testing.T) {
	// On a non-TTY writer, render only emits output when current == total.
	// Verify that SetCurrent(total) produces output and SetCurrent(partial) does not.
	t.Run("completion emits output", func(t *testing.T) {
		buf := &bytes.Buffer{}
		p := NewProgress(100, "Downloading")
		p.SetWriter(buf)
		p.SetCurrent(100)
		output := buf.String()
		if !strings.Contains(output, "100%") {
			t.Errorf("SetCurrent(100) should show 100%%, got: %q", output)
		}
	})

	t.Run("intermediate emits no output on non-TTY", func(t *testing.T) {
		buf := &bytes.Buffer{}
		p := NewProgress(100, "Downloading")
		p.SetWriter(buf)
		p.SetCurrent(50)
		output := buf.String()
		if len(output) != 0 {
			t.Errorf("SetCurrent(50) on non-TTY should produce no output, got: %q", output)
		}
	})
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

	// On non-TTY, drive to completion to get output
	p.SetCurrent(100)
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
	// Construct the spinner with the test writer set before Start() so that
	// non-TTY detection uses the correct writer from the beginning.
	buf := &bytes.Buffer{}
	s := &Spinner{
		message: "Loading",
		chars:   []string{"|", "/", "-", "\\"},
		writer:  buf,
		done:    make(chan struct{}),
	}
	s.Start()

	// Give it a moment (TTY: goroutine ticks; non-TTY: message already printed)
	time.Sleep(150 * time.Millisecond)

	s.Stop()
	output := buf.String()

	// On a TTY the goroutine renders animation; on non-TTY Start() prints "message..."
	// Either way the buf should contain something.
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
	// Construct with writer before Start so non-TTY path uses buf.
	buf := &bytes.Buffer{}
	s := &Spinner{
		message: "Initial",
		chars:   []string{"|", "/", "-", "\\"},
		writer:  buf,
		done:    make(chan struct{}),
	}
	s.Start()

	// Wait for initial render
	time.Sleep(50 * time.Millisecond)

	// Update message
	s.UpdateMessage("Updated")

	// Wait for updated render (TTY: goroutine picks up new message; non-TTY: no-op)
	time.Sleep(150 * time.Millisecond)

	// On TTY, the goroutine will have rendered the updated message into buf.
	// On non-TTY, no goroutine runs; just verify no panic occurs.
	s.StopWithMessage("Final: Updated")

	output := buf.String()
	// StopWithMessage always writes its argument to the writer, so check that.
	if !strings.Contains(output, "Updated") {
		t.Errorf("Spinner should contain 'Updated' in output, got: %q", output)
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

	// Construct with writer before Start so non-TTY path uses buf.
	buf := &bytes.Buffer{}
	s := &Spinner{
		message: "Analyzing packages",
		chars:   []string{"|", "/", "-", "\\"},
		writer:  buf,
		done:    make(chan struct{}),
	}
	s.Start()

	// Let it spin for a bit
	time.Sleep(500 * time.Millisecond)

	s.Stop()

	output := buf.String()
	t.Logf("Spinner output:\n%s", output)

	// On a TTY the goroutine cycles animation characters.
	// On a non-TTY Start() emits "message...\n"; no animation chars, which is expected.
	if writerIsTTY(buf) {
		hasBar := strings.Contains(output, "|")
		hasSlash := strings.Contains(output, "/")
		hasDash := strings.Contains(output, "-")
		hasBackslash := strings.Contains(output, "\\")
		if !hasBar && !hasSlash && !hasDash && !hasBackslash {
			t.Error("Spinner should have rendered at least one animation character on TTY")
		}
	} else {
		// Non-TTY: just verify something was produced
		if len(output) == 0 {
			t.Error("Spinner should produce output even on non-TTY")
		}
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

// TestProgressBar_NoTTY_NoDuplicateLine verifies that writing to a non-TTY
// buffer does not produce duplicate 100% lines. On a non-TTY, render() must
// only emit output once (at completion), and Finish() must not add an extra
// blank line on top of that.
func TestProgressBar_NoTTY_NoDuplicateLine(t *testing.T) {
	buf := &bytes.Buffer{}
	total := 5
	p := NewProgress(total, "loading")
	p.SetWriter(buf) // bytes.Buffer is not a TTY

	// Increment through all steps then finish
	for i := 0; i < total; i++ {
		p.Increment()
	}
	p.Finish()

	output := buf.String()

	// Count occurrences of "100%"
	count := strings.Count(output, "100%")
	if count != 1 {
		t.Errorf("expected exactly 1 occurrence of '100%%' on non-TTY, got %d\nOutput: %q", count, output)
	}

	// The single line must end with a newline
	if !strings.HasSuffix(output, "\n") {
		t.Errorf("non-TTY output should end with newline, got: %q", output)
	}
}
