package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"
)

const (
	verbosityQuiet   = iota // only errors and the analyze report
	verbositySpinner        // a spinner while working
	verbosityFiles          // spinner, number of files and the current file
	verbosityStats          // like verbosityFiles plus hashes per second and a runtime summary
)

// clearLine moves to the line start and clears it
const clearLine = "\r\033[K"

// spinnerFrames pulse from a dot to a full star and back, "*" instead of "✳" because some fonts draw that one as a wide emoji
var spinnerFrames = []rune("·✢*✶✻✽✻✶*✢")

// spinnerInterval is the time between two frames, slow enough that the star pulses instead of flickering
const spinnerInterval = 150 * time.Millisecond

// colors for the progress line, only used when the output is a terminal
const (
	colorSpinner = "\033[38;2;0;99;175m" // DMK blue #0063af
	colorDim     = "\033[2m"
	colorReset   = "\033[0m"
)

// maxPathLength keeps the progress line on one terminal line
const maxPathLength = 100

// verbosityFlag adds step to level every time the flag is given, so -v -v is the same as -vv.
type verbosityFlag struct {
	level *int
	step  int
}

func (v verbosityFlag) String() string {
	if v.level == nil {
		return "0"
	}
	return strconv.Itoa(*v.level)
}

func (v verbosityFlag) Set(s string) error {
	on, err := strconv.ParseBool(s)
	if err != nil {
		return err
	}
	if on {
		*v.level += v.step
	}
	return nil
}

func (v verbosityFlag) IsBoolFlag() bool {
	return true
}

// progress shows what rescan and analyze are working on, depending on the verbosity.
type progress struct {
	level   int
	out     io.Writer
	color   bool
	mu      sync.Mutex // serializes writes to out
	count   atomic.Int64
	hashed  atomic.Int64
	current atomic.Pointer[string]
	label   string
	started time.Time
	stop    chan struct{}
	done    chan struct{}
}

func newProgress(level int, out io.Writer) *progress {
	return &progress{
		level: level,
		out:   out,
		color: isTerminal(out),
	}
}

// isTerminal reports whether out is a terminal, so redirected output gets no color codes.
func isTerminal(out io.Writer) bool {
	f, ok := out.(*os.File)
	if !ok {
		return false
	}

	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// Start redraws the progress line every spinnerInterval until Stop is called, label says what is being done.
func (p *progress) Start(label string) {
	if p.level < verbositySpinner {
		return
	}

	p.label = label
	p.started = time.Now()
	p.stop = make(chan struct{})
	p.done = make(chan struct{})

	go func() {
		defer close(p.done)

		ticker := time.NewTicker(spinnerInterval)
		defer ticker.Stop()

		for i := 0; ; i++ {
			p.draw(spinnerFrames[i%len(spinnerFrames)])

			select {
			case <-p.stop:
				return
			case <-ticker.C:
			}
		}
	}()
}

// Stop ends the progress line and clears it, so following output starts on a clean line.
func (p *progress) Stop() {
	if p.stop == nil {
		return
	}

	close(p.stop)
	<-p.done
	p.stop = nil

	p.mu.Lock()
	defer p.mu.Unlock()
	fmt.Fprint(p.out, clearLine)

	if p.level >= verbosityStats {
		elapsed := time.Since(p.started)
		fmt.Fprintf(p.out, "%s done: %d files hashed in %s, %s\n",
			p.label, p.hashed.Load(), elapsed.Round(10*time.Millisecond), p.rate(elapsed))
	}
}

// File records the file a worker starts on, it is safe for concurrent use.
func (p *progress) File(path string) {
	if p.level < verbosityFiles {
		return
	}

	p.count.Add(1)
	p.current.Store(&path)
}

// Hashed records a successfully hashed file, it is safe for concurrent use.
func (p *progress) Hashed() {
	if p.level < verbosityStats {
		return
	}

	p.hashed.Add(1)
}

// rate returns the average hashes per second since Start
func (p *progress) rate(elapsed time.Duration) string {
	if elapsed <= 0 {
		return "0 hashes/s"
	}

	return fmt.Sprintf("%.0f hashes/s", float64(p.hashed.Load())/elapsed.Seconds())
}

// Error prints err on its own line, the progress line is redrawn below it.
func (p *progress) Error(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.level >= verbositySpinner {
		fmt.Fprint(p.out, clearLine)
	}
	fmt.Fprintln(p.out, err)
}

func (p *progress) draw(spin rune) {
	p.mu.Lock()
	defer p.mu.Unlock()

	elapsed := fmt.Sprintf("(%ds)", int(time.Since(p.started).Seconds()))
	line := fmt.Sprintf("%s %s… %s", p.paint(colorSpinner, string(spin)), p.label, p.paint(colorDim, elapsed))

	if p.level >= verbosityFiles {
		current := ""
		if c := p.current.Load(); c != nil {
			current = shortenPath(*c, maxPathLength)
		}

		stats := ""
		if p.level >= verbosityStats {
			stats = " · " + p.rate(time.Since(p.started))
		}

		line += p.paint(colorDim, fmt.Sprintf(" · %d files%s · %s", p.count.Load(), stats, current))
	}

	fmt.Fprint(p.out, clearLine+line)
}

// paint colors s if the output is a terminal
func (p *progress) paint(color, s string) string {
	if !p.color {
		return s
	}

	return color + s + colorReset
}

// shortenPath keeps the end of path, which names the file, if it is longer than max bytes.
func shortenPath(path string, max int) string {
	if len(path) <= max {
		return path
	}

	start := len(path) - max + 3
	for start < len(path) && !utf8.RuneStart(path[start]) {
		start++
	}

	return "..." + path[start:]
}
