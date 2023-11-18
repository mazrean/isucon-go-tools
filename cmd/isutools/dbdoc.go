package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/mazrean/isucon-go-tools/internal/dbdoc"
)

var (
	dbDocFlagSet   = flag.NewFlagSet("dbdoc", flag.ExitOnError)
	dst            string
	wd             string
	ignores        sliceString
	ignorePrefixes sliceString
)

func init() {
	dbDocFlagSet.StringVar(&dst, "dst", "./dbdoc.md", "destination file")
	dbDocFlagSet.Var(&ignores, "ignore", "ignore function")
	dbDocFlagSet.Var(&ignorePrefixes, "ignorePrefix", "ignore function")
}

func dbDoc(args []string) error {
	err := dbDocFlagSet.Parse(args)
	if err != nil {
		return fmt.Errorf("failed to parse flag: %w", err)
	}

	wd, err = os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	err = dbdoc.Run(dbdoc.Config{
		WorkDir:             wd,
		BuildArgs:           dbDocFlagSet.Args(),
		IgnoreFuncs:         ignores,
		IgnoreFuncPrefixes:  ignorePrefixes,
		DestinationFilePath: dst,
	})
	if err != nil {
		return fmt.Errorf("failed to run dbdoc: %w", err)
	}

	return nil
}
