package lister

import (
	"bytes"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"github.com/DMKEBUSINESSGMBH/dmkhunter/config"
	"github.com/olekukonko/tablewriter"
	"go.etcd.io/bbolt"
	"os"
	"strings"
	"time"
)

// List prints the stored file hashes as a table, args are the flags after the "list" command.
func List(conf *config.Config, args []string) error {
	flags := flag.NewFlagSet("list", flag.ExitOnError)
	database := flags.String("db", "", "database file (default: the databases of all presets)")
	prefix := flags.String("prefix", "", "only list paths starting with this prefix")
	contains := flags.String("contains", "", "only list paths containing this text")
	limit := flags.Int("limit", 0, "maximum number of records, 0 lists all")
	flags.Parse(args)

	databases := []string{*database}
	if *database == "" {
		databases = conf.GetDatabases()
	}

	// the database column is only needed if records come from more than one file
	withDatabase := len(databases) > 1

	writer := tablewriter.NewWriter(os.Stdout)
	if withDatabase {
		writer.Header("Database", "Path", "Hash")
	} else {
		writer.Header("Path", "Hash")
	}

	count := 0

	for _, name := range databases {
		if *limit > 0 && count >= *limit {
			break
		}

		err := ReadHashes(name, *prefix, func(path string, hash []byte) bool {
			if *contains != "" && !strings.Contains(path, *contains) {
				return true
			}

			row := []string{path, hex.EncodeToString(hash)}
			if withDatabase {
				row = append([]string{name}, row...)
			}
			writer.Append(row)
			count++

			return *limit == 0 || count < *limit
		})

		if err != nil {
			return err
		}
	}

	writer.Render()
	fmt.Printf("%d records\n", count)

	return nil
}

// ReadHashes calls fn for every stored path starting with prefix, in path order, until fn returns false.
func ReadHashes(databasename string, prefix string, fn func(path string, hash []byte) bool) error {
	db, err := bbolt.Open(databasename, 0600, &bbolt.Options{ReadOnly: true, Timeout: 5 * time.Second})

	if err != nil {
		return fmt.Errorf("could not open database %s: %w", databasename, err)
	}
	defer db.Close()

	return db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("file_hashes"))

		if b == nil {
			return errors.New("database " + databasename + " has no file hashes, run rescan first")
		}

		c := b.Cursor()
		p := []byte(prefix)

		for k, v := c.Seek(p); k != nil && bytes.HasPrefix(k, p); k, v = c.Next() {
			if !fn(string(k), v) {
				break
			}
		}

		return nil
	})
}
