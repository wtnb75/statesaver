package main

import "fmt"

type VersionCmd struct {
	FullVersion bool `long:"full-version"`
}

var (
	version = "dev"
	commit  = "dummy_hash"
	date    = "dummy_date"
)

func (cmd VersionCmd) Execute(args []string) error {
	if cmd.FullVersion {
		fmt.Println("statesaver", version, "hash", commit, "build", date)
		return nil
	}
	fmt.Println("statesaver", version)
	return nil
}
