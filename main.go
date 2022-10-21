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
	"time"
)

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

func saveHashes(databasename string, scanner scanner2.Scanner) error {
	db, _ := bbolt.Open(databasename, 0600, nil)
	defer db.Close()

	return db.Batch(func(tx *bbolt.Tx) error {
		b, _ := tx.CreateBucketIfNotExists([]byte("file_hashes"))

		for p, f := range scanner.Scan() {
			hash, err := f.Hash()

			if err != nil {
				panic(err)
			}

			if err := b.Put([]byte(p), hash); err != nil {
				return err
			}
		}

		//fmt.Printf("\n============================\n%v", b.Bucket([]byte("files")))
		//json.NewEncoder(os.Stderr).Encode(b.Stats())

		//tx.Bucket([]byte("file_hashes")).ForEach(func(k, v []byte) error {
		//	fmt.Printf("%s _ %s\n", k, hex.EncodeToString(v))
		//
		//	return nil
		//})

		return nil
	})
}

func analyze(conf *config.Config) {
	stack := model.ViolationStack{}

	for _, preset := range conf.Presets {
		scanner := scanner2.NewScanner(preset.Paths, &preset.Ignores)
		var analyzers []analyzer.Analyzer

		if preset.Database != nil {
			dbanalyzer, err := analyzer.NewDatabaseAnalyzer(*preset.Database)

			if err != nil {
				return
			}
			analyzers = append(analyzers, dbanalyzer)
		}

		for _, paths := range scanner.Scan() {
			for _, a := range analyzers {
				err := a.Analyze(*paths, &stack)
				if err != nil {
					fmt.Println(err)
				}
			}
		}
	}

	if stack.Count() > 0 {
		reporters := conf.GetReporters()

		if err := reporters.Send(stack); err != nil {
			panic(err)
		}
	}
}

func main() {
	var configFile string
	flag.StringVar(&configFile, "config", ".hunter.conf", "")
	flag.Parse()

	conf, err := config.LoadConfiguration(configFile)

	if err != nil {
		panic(err)
	}

	if len(os.Args) <= 1 {
		help()
		os.Exit(0)
	}

	switch os.Args[1] {
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
