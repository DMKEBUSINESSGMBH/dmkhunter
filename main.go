package main

import (
	"flag"
	"fmt"
	"github.com/DMKEBUSINESSGMBH/dmkhunter/config"
	scanner2 "github.com/DMKEBUSINESSGMBH/dmkhunter/scanner"
	"go.etcd.io/bbolt"
	"os"
)

func rescan(conf *config.Config) {
	scanner := scanner2.NewScanner([]string{"**/*.go"})
	db, _ := bbolt.Open("hunter.db", 0600, nil)
	defer db.Close()

	err := db.Batch(func(tx *bbolt.Tx) error {
		b, _ := tx.CreateBucketIfNotExists([]byte("files"))

		for p, f := range scanner.Scan() {
			hash, err := f.Hash()

			if err != nil {
				panic(err)
			}

			if err := b.Put([]byte(p), hash); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		panic(err)
	}
}

/*
	func analyze(conf *config.Config) {
		scanner := scanner2.NewScanner([]string{"Foo", "Bar"})
		paths := scanner.Scan()

		var analyzers []analyzer.Analyzer
		stack := model.ViolationStack{}

		for _, a := range analyzers {
			go func() {
				// for _, file := range paths {
					// go a.Analyze(&file, stack)
				// }
			}()
		}



		if err := conf.GetReporters().Send(stack); err != nil {
		 	panic(err)
		}
	}
*/
func main() {
	var configFile string
	flag.StringVar(&configFile, "config", ".hunter.conf", "")
	flag.Parse()

	conf, err := config.LoadConfiguration(configFile)

	if err != nil {
		panic(err)
	}

	switch os.Args[1] {
	case "scan":
	case "rescan":
		rescan(conf)
		break
	case "analyze":
	default:
		// analyze(conf)
	}
}
