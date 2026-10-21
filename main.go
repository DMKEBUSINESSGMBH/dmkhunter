package main

import (
	"flag"
	"fmt"
	"github.com/DMKEBUSINESSGMBH/dmkhunter/analyzer"
	"github.com/DMKEBUSINESSGMBH/dmkhunter/config"
	"github.com/DMKEBUSINESSGMBH/dmkhunter/model"
	scanner2 "github.com/DMKEBUSINESSGMBH/dmkhunter/scanner"
	"go.etcd.io/bbolt"
	"os"
	"runtime"
	"sort"
	"sync"
	"time"
)

// number of entries written per bbolt transaction during rescan
const writeBatchSize = 10000

var workers = runtime.NumCPU()

func rescan(conf *config.Config) {

	for _, preset := range conf.Presets {
		scanner := scanner2.NewScanner(preset.Paths, &preset.Ignores)
		var databasename string

		if preset.Database != nil {
			databasename = *preset.Database
		} else {
			databasename = "hunter.db"
		}

		err := saveHashes(databasename, scanner)

		if err != nil {
			panic(err)
		}
	}
}

type fileHash struct {
	path string
	hash []byte
}

func saveHashes(databasename string, scanner scanner2.Scanner) error {
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
				hash, err := f.Hash()

				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					continue
				}

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

func analyze(conf *config.Config) {
	stack := model.ViolationStack{}

	for _, preset := range conf.Presets {
		if preset.Database == nil {
			continue
		}

		dbanalyzer, err := analyzer.NewDatabaseAnalyzer(*preset.Database)

		if err != nil {
			panic(err)
		}

		scanner := scanner2.NewScanner(preset.Paths, &preset.Ignores)
		err = analyzeFiles(scanner, []analyzer.Analyzer{dbanalyzer}, &stack)
		dbanalyzer.Close()

		if err != nil {
			panic(err)
		}
	}

	if stack.Count() > 0 {
		reporters := conf.GetReporters()

		if err := reporters.Send(stack); err != nil {
			panic(err)
		}
	}
}

// analyzeFiles runs the analyzers on all scanned files in parallel and adds the violations to stack.
func analyzeFiles(scanner scanner2.Scanner, analyzers []analyzer.Analyzer, stack *model.ViolationStack) error {
	files := make(chan *model.File, 1024)
	// one stack per worker, ViolationStack is not safe for concurrent use
	stacks := make([]model.ViolationStack, workers)
	var wg sync.WaitGroup

	for i := range stacks {
		wg.Go(func() {
			for f := range files {
				for _, a := range analyzers {
					if err := a.Analyze(*f, &stacks[i]); err != nil {
						fmt.Println(err)
					}
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

func main() {
	var configFile string
	flag.StringVar(&configFile, "config", ".hunter.conf", "")
	flag.Parse()

	conf, err := config.LoadConfiguration(configFile)

	if err != nil {
		panic(err)
	}

	if flag.NArg() == 0 {
		help()
		os.Exit(0)
	}

	switch flag.Arg(0) {
	case "scan":
		//
	case "rescan":
		go spinner(100 * time.Millisecond)
		rescan(conf)
		break
	case "analyze":
		go spinner(100 * time.Millisecond)
		analyze(conf)
	default:
		help()
	}
}

func spinner(delay time.Duration) {
	for {
		for _, r := range `-\|/` {
			fmt.Printf("\r%c\r", r)
			time.Sleep(delay)
		}
	}
}

func help() {
	help_info := `DMKHunter help:

available arguments:
------------------------------------------------------------------------------
rescan	- rescan the configured directories and rewrite md5 hashes to database
scan	- 
analyze	- check all hashes for files in configured directories and report
	`
	fmt.Println(help_info)
}
