package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/DMKEBUSINESSGMBH/dmkhunter/analyzer"
	"github.com/DMKEBUSINESSGMBH/dmkhunter/config"
	"github.com/DMKEBUSINESSGMBH/dmkhunter/lister"
	"github.com/DMKEBUSINESSGMBH/dmkhunter/model"
	scanner2 "github.com/DMKEBUSINESSGMBH/dmkhunter/scanner"
	"go.etcd.io/bbolt"
)

// number of entries written per bbolt transaction during rescan
const writeBatchSize = 10000

var workers = runtime.NumCPU()

func rescan(conf *config.Config, prog *progress) error {
	for _, preset := range conf.Presets {
		scanner := scanner2.NewScanner(preset.Paths, &preset.Ignores)

		if err := saveHashes(preset.GetDatabase(), scanner, prog); err != nil {
			return err
		}
	}

	return nil
}

// confirmDeleteDatabases asks for every existing database whether it should be deleted before the rescan.
// Anything but y or yes keeps the database, so a rescan without a terminal never deletes it.
func confirmDeleteDatabases(databases []string, in io.Reader, out io.Writer) error {
	answers := bufio.NewReader(in)

	for _, name := range databases {
		if _, err := os.Stat(name); errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			return err
		}

		fmt.Fprintf(out, "Database %s already exists, delete it before rescan? [y/N] ", name)
		answer, err := answers.ReadString('\n')

		if err != nil && err != io.EOF {
			return err
		}

		switch strings.ToLower(strings.TrimSpace(answer)) {
		case "y", "yes":
			if err := os.Remove(name); err != nil {
				return err
			}
			fmt.Fprintf(out, "deleted %s\n", name)
		default:
			fmt.Fprintf(out, "keeping %s\n", name)
		}
	}

	return nil
}

type fileHash struct {
	path string
	hash []byte
}

func saveHashes(databasename string, scanner scanner2.Scanner, prog *progress) error {
	db, err := bbolt.Open(databasename, 0600, &bbolt.Options{Timeout: 5 * time.Second})

	if err != nil {
		return err
	}
	defer db.Close()

	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("file_hashes"))
		return err
	})

	if err != nil {
		return err
	}

	files := make(chan *model.File, 1024)
	hashes := make(chan fileHash, 1024)
	var wg sync.WaitGroup

	for range workers {
		wg.Go(func() {
			for f := range files {
				prog.File(f.Path)
				hash, err := f.Hash()

				if err != nil {
					prog.Error(err)
					continue
				}

				prog.Hashed()
				hashes <- fileHash{path: f.Path, hash: hash}
			}
		})
	}

	var walkErr error

	go func() {
		walkErr = scanner.Walk(func(f *model.File) error {
			files <- f
			return nil
		})
		close(files)
		wg.Wait()
		close(hashes)
	}()

	var writeErr error
	batch := make([]fileHash, 0, writeBatchSize)

	flush := func() {
		if writeErr != nil || len(batch) == 0 {
			return
		}

		// bbolt inserts sorted keys faster
		sort.Slice(batch, func(i, j int) bool { return batch[i].path < batch[j].path })

		writeErr = db.Update(func(tx *bbolt.Tx) error {
			b := tx.Bucket([]byte("file_hashes"))

			for _, e := range batch {
				if err := b.Put([]byte(e.path), e.hash); err != nil {
					return err
				}
			}

			return nil
		})
		batch = batch[:0]
	}

	// keep draining after a write error so the workers don't block
	for e := range hashes {
		batch = append(batch, e)

		if len(batch) == writeBatchSize {
			flush()
		}
	}
	flush()

	if walkErr != nil {
		return walkErr
	}

	return writeErr
}

// analyze checks all presets with a database and returns the violations found, reporting is up to the caller.
func analyze(conf *config.Config, prog *progress) (model.ViolationStack, error) {
	stack := model.ViolationStack{}

	for _, preset := range conf.Presets {
		if preset.Database == nil {
			continue
		}

		dbanalyzer, err := analyzer.NewDatabaseAnalyzer(*preset.Database)

		if err != nil {
			return stack, err
		}

		scanner := scanner2.NewScanner(preset.Paths, &preset.Ignores)
		err = analyzeFiles(scanner, []analyzer.Analyzer{dbanalyzer}, &stack, prog)
		dbanalyzer.Close()

		if err != nil {
			return stack, err
		}
	}

	return stack, nil
}

// analyzeFiles runs the analyzers on all scanned files in parallel and adds the violations to stack.
func analyzeFiles(scanner scanner2.Scanner, analyzers []analyzer.Analyzer, stack *model.ViolationStack, prog *progress) error {
	files := make(chan *model.File, 1024)
	// one stack per worker, ViolationStack is not safe for concurrent use
	stacks := make([]model.ViolationStack, workers)
	var wg sync.WaitGroup

	for i := range stacks {
		wg.Go(func() {
			for f := range files {
				prog.File(f.Path)
				ok := true

				for _, a := range analyzers {
					if err := a.Analyze(*f, &stacks[i]); err != nil {
						prog.Error(err)
						ok = false
					}
				}

				if ok {
					prog.Hashed()
				}
			}
		})
	}

	err := scanner.Walk(func(f *model.File) error {
		files <- f
		return nil
	})
	close(files)
	wg.Wait()

	for i := range stacks {
		for _, v := range stacks[i].All() {
			stack.Add(v)
		}
	}

	return err
}

// exit codes, 1 is only used by analyze to signal violations
const (
	exitViolations = 1
	exitError      = 2
)

// fail prints err and exits, so errors can't be mistaken for found violations.
func fail(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(exitError)
}

// splitGlobalFlags moves the flags defined in globals out of args, wherever they are,
// so "analyze -vv" works like "-vv analyze". The other args keep their order for the command.
// Everything after "--" is left to the command.
func splitGlobalFlags(args []string, globals *flag.FlagSet) (flags []string, rest []string) {
	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "--" {
			return flags, append(rest, args[i+1:]...)
		}

		name, isFlag := strings.CutPrefix(arg, "--")
		if !isFlag {
			name, isFlag = strings.CutPrefix(arg, "-")
		}
		name, _, inline := strings.Cut(name, "=")

		f := globals.Lookup(name)
		if !isFlag || name == "" || f == nil {
			rest = append(rest, arg)
			continue
		}

		flags = append(flags, arg)

		// a non-bool flag without "=value" takes the next arg as its value
		if b, ok := f.Value.(interface{ IsBoolFlag() bool }); (!ok || !b.IsBoolFlag()) && !inline && i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}

	return flags, rest
}

func main() {
	var configFile string
	verbosity := verbosityQuiet
	flag.StringVar(&configFile, "config", ".hunter.conf", "")
	flag.Var(verbosityFlag{&verbosity, 1}, "v", "show a spinner, -v -v also shows the current file")
	flag.Var(verbosityFlag{&verbosity, 2}, "vv", "show a spinner and the current file")
	flag.Var(verbosityFlag{&verbosity, 3}, "vvv", "like -vv plus hashes per second and the runtime")

	globals, args := splitGlobalFlags(os.Args[1:], flag.CommandLine)
	flag.CommandLine.Parse(globals)

	conf, err := config.LoadConfiguration(configFile)

	if err != nil {
		fail(err)
	}

	if len(args) == 0 {
		help()
		os.Exit(0)
	}

	prog := newProgress(verbosity, os.Stderr)

	switch args[0] {
	case "scan":
		//
	case "rescan":
		// ask before the progress starts, it would overwrite the prompt
		if err := confirmDeleteDatabases(conf.GetDatabases(), os.Stdin, os.Stdout); err != nil {
			fail(err)
		}

		prog.Start("Rescanning")
		err := rescan(conf, prog)
		prog.Stop()

		if err != nil {
			fail(err)
		}
	case "analyze":
		prog.Start("Analyzing")
		stack, err := analyze(conf, prog)
		prog.Stop()

		if err != nil {
			fail(err)
		}

		if stack.Count() > 0 {
			reporters := conf.GetReporters()

			if err := reporters.Send(stack); err != nil {
				fail(err)
			}
			os.Exit(exitViolations)
		}
	case "list":
		if err := lister.List(conf, args[1:]); err != nil {
			fail(err)
		}
	default:
		help()
	}
}

func help() {
	help_info := `DMKHunter help:

usage: dmkhunter [-config <file>] [-v|-vv|-vvv] <argument>

available arguments:
------------------------------------------------------------------------------
rescan	- rescan the configured directories and rewrite md5 hashes to database
scan	-
analyze	- check all hashes for files in configured directories and report
list	- show stored hashes as a table, options:
	  -prefix <path>   only paths starting with <path>
	  -contains <text> only paths containing <text>
	  -limit <n>       show at most <n> records
	  -db <file>       read <file> instead of the configured databases

verbosity (progress goes to stderr):
------------------------------------------------------------------------------
(none)	- print only errors and the analyze report
-v	- show a spinner while working
-vv	- show a spinner, the number of files and the current file
-vvv	- like -vv plus hashes per second, and the runtime when done

exit codes:
------------------------------------------------------------------------------
0	- success, analyze found no violations
1	- analyze found violations
2	- error, e.g. missing config or database
	`
	fmt.Println(help_info)
}
