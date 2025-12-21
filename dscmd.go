package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
)

type LsTree struct {
}

func (cmd *LsTree) Execute(args []string) error {
	init_log()
	root := NewDatastore(option.Datadir)
	err := root.Walk(func(e FileEntry) error {
		fmt.Println(e.Name, e.Size, e.Timestamp, e.Locked)
		return nil
	})
	if err != nil {
		slog.Error("walk error", "error", err, "root", root.RootDir)
	}
	return err
}

type Cat struct {
	JSON bool `short:"j" long:"json" description:"read as json, output compat json"`
}

func (cmd *Cat) Execute(args []string) error {
	init_log()
	root := NewDatastore(option.Datadir)
	for _, v := range args {
		if !cmd.JSON {
			if err := root.Read(v, os.Stdout); err != nil {
				slog.Error("read error", "error", err, "name", v)
				return err
			}
		} else {
			buf := bytes.Buffer{}
			if err := root.Read(v, &buf); err != nil {
				slog.Error("read error", "error", err, "name", v)
				return err
			}
			enc := json.NewEncoder(os.Stdout)
			if err := enc.Encode(root.ParseJSON(buf.String())); err != nil {
				slog.Error("encode error", "error", err, "name", v)
				return err
			}
		}
	}
	return nil
}

type Put struct {
	Prefix string `short:"p" long:"prefix" description:"output prefix"`
	Lock   string `long:"lock" description:"lock string"`
	Hash   bool   `long:"hash" description:"using hash"`
}

type LockStruct struct {
	ID string
}

func (cmd *Put) Execute(args []string) error {
	init_log()
	root := NewDatastore(option.Datadir)
	for _, v := range args {
		fp, err := os.Open(v)
		if err != nil {
			slog.Error("open file", "name", v, "error", err)
			continue
		}
		defer fp.Close()
		err = root.Write(cmd.Prefix+v, fp, []byte{}, cmd.Lock)
		if err != nil {
			slog.Error("put failed", "error", err, "name", cmd.Prefix+v)
		}
	}
	return nil
}

type History struct {
}

func (cmd *History) Execute(args []string) error {
	init_log()
	root := NewDatastore(option.Datadir)
	for _, v := range args {
		fmt.Println(v)
		for _, e := range root.History(v) {
			fmt.Println("  ", e.Name, e.Size, e.Timestamp, e.Locked)
		}
	}
	return nil
}

type Prune struct {
	Keep int  `short:"k" long:"keep" description:"keep generations" default:"5"`
	Dry  bool `short:"n" long:"dry-run" description:"do not remove"`
	All  bool `short:"a" long:"all" description:"walk and prune"`
}

func (cmd *Prune) Execute(args []string) error {
	init_log()
	root := NewDatastore(option.Datadir)
	if cmd.All {
		return root.Walk(func(e FileEntry) error {
			slog.Info("try prune", "name", e.Name)
			return root.Prune(e.Name, cmd.Keep, cmd.Dry)
		})
	} else {
		for _, v := range args {
			fmt.Println(v)
			if err := root.Prune(v, cmd.Keep, cmd.Dry); err != nil {
				slog.Error("prune failed", "name", v, "error", err)
				return err
			}
		}
	}
	return nil
}

type HistoryCat struct {
	File string `short:"f" long:"file" description:"file name"`
}

func (cmd *HistoryCat) Execute(args []string) error {
	init_log()
	root := NewDatastore(option.Datadir)
	for _, v := range args {
		if fp, err := root.ReadHistory(cmd.File, v); err != nil {
			slog.Error("read failed", "name", cmd.File, "history", v, "error", err)
		} else {
			if written, err := io.Copy(os.Stdout, fp); err != nil {
				slog.Error("part read", "name", cmd.File, "history", v, "written", written, "error", err)
			}
			fp.Close()
		}
	}
	return nil
}

type HistoryRollback struct {
	File    string `short:"f" long:"file" description:"file name"`
	History string `short:"t" long:"history" description:"rollback to"`
}

func (cmd *HistoryRollback) Execute(args []string) error {
	init_log()
	root := NewDatastore(option.Datadir)
	return root.Rollback(cmd.File, cmd.History)
}
