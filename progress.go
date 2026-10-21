package main

import (
	"fmt"
	"io"
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
)

// clearLine moves to the line start and clears it
const clearLine = "\r\033[K"

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
	mu      sync.Mutex // serializes writes to out
	count   atomic.Int64
	current atomic.Pointer[string]
	stop    chan struct{}
	done    chan struct{}
}

func newProgress(level int, out io.Writer) *progress {
	return &progress{
		level: level,
		out:   out,
	}
}

// Start redraws the progress line every 100ms until Stop is called.
func (p *progress) Start() {
	if p.level < verbositySpinner {
		return
	}

	p.stop = make(chan struct{})
	p.done = make(chan struct{})

	go func() {
		defer close(p.done)

		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for i := 0; ; i++ {
			p.draw(`-\|/`[i%4])

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
}

// File records the file a worker starts on, it is safe for concurrent use.
func (p *progress) File(path string) {
	if p.level < verbosityFiles {
		return
	}

	p.count.Add(1)
	p.current.Store(&path)
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

func (p *progress) draw(spin byte) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.level < verbosityFiles {
		fmt.Fprintf(p.out, "\r%c", spin)
		return
	}

	current := ""
	if c := p.current.Load(); c != nil {
		current = shortenPath(*c, maxPathLength)
	}

	fmt.Fprintf(p.out, "%s%c %d files %s", clearLine, spin, p.count.Load(), current)
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
