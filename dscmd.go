package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/confluentinc/go-editor"
)

// LsTree lists the files in the datastore
type LsTree struct {
}

func (cmd *LsTree) do1(root Datastore, prefix string) error {
	err := root.Walk(prefix, func(e FileEntry) error {
		locked := ""
		if e.Locked {
			locked = " (locked)"
		}
		fmt.Printf("%s %6d %s%s\n", e.Timestamp.Format(time.RFC3339), e.Size, e.Name, locked)
		return nil
	})
	if err != nil {
		slog.Error("walk error", "error", err, "root", root.RootDir)
	}
	return err
}

func (cmd *LsTree) Execute(args []string) error {
	init_log()
	root := NewDatastore(option.Datadir)
	if len(args) == 0 {
		args = append(args, "/")
	}
	for _, v := range args {
		if err := cmd.do1(root, v); err != nil {
			return err
		}
	}
	return nil
}

// Cat outputs the contents of files in the datastore
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

// Put stores files into the datastore
type Put struct {
	Prefix string `short:"p" long:"prefix" description:"output prefix"`
	Lock   string `long:"lock" description:"lock string"`
	Hash   bool   `long:"hash" description:"using hash"`
	NoJson bool   `long:"no-json" description:"do not validate JSON"`
}

// LockStruct represents a lock structure
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
		if !cmd.NoJson {
			buf := &bytes.Buffer{}
			if _, err := io.Copy(buf, fp); err != nil {
				slog.Error("read file", "name", v, "error", err)
				continue
			}
			if root.ParseJSON(buf.String()) == nil {
				slog.Error("invalid json", "name", v)
				continue
			}
			// Reset file pointer
			fp.Seek(0, io.SeekStart)
		}
		err = root.Write(cmd.Prefix+v, fp, []byte{}, cmd.Lock)
		if err != nil {
			slog.Error("put failed", "error", err, "name", cmd.Prefix+v)
		}
	}
	return nil
}

// History lists the history of files in the datastore
type History struct {
}

func (cmd *History) Execute(args []string) error {
	init_log()
	root := NewDatastore(option.Datadir)
	for _, v := range args {
		fmt.Println(v)
		for _, e := range root.History(v) {
			current := ""
			if e.Locked {
				current = " (current)"
			}
			fmt.Printf("%s %6d %s%s\n", e.Timestamp.Format(time.RFC3339), e.Size, e.Name, current)
		}
	}
	return nil
}

// Prune removes old history entries from the datastore
type Prune struct {
	Keep int  `short:"k" long:"keep" description:"keep generations" default:"5"`
	Dry  bool `short:"n" long:"dry-run" description:"do not remove"`
	All  bool `short:"a" long:"all" description:"walk and prune"`
}

func (cmd *Prune) Execute(args []string) error {
	init_log()
	root := NewDatastore(option.Datadir)
	if len(args) == 0 {
		args = append(args, "/")
	}
	if cmd.All {
		for _, v := range args {
			if err := root.Walk(v, func(e FileEntry) error {
				slog.Info("try prune", "name", e.Name, "keep", cmd.Keep, "dry", cmd.Dry)
				return root.Prune(e.Name, cmd.Keep, cmd.Dry)
			}); err != nil {
				return err
			}
		}
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

// HistoryCat outputs the contents of historical versions of files
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

// HistoryRollback rolls back a file to a specified historical version
type HistoryRollback struct {
	File    string `short:"f" long:"file" description:"file name" required:"true"`
	History string `short:"t" long:"history" description:"rollback to" required:"true"`
}

func (cmd *HistoryRollback) Execute(args []string) error {
	init_log()
	root := NewDatastore(option.Datadir)
	return root.Rollback(cmd.File, cmd.History)
}

type chkjson struct {
	editor.Schema
	ds Datastore
}

// ValidateBytes simply checks if the provided data is valid JSON
func (s *chkjson) ValidateBytes(data []byte) error {
	if s.ds.ParseJSON(string(data)) == nil {
		return fmt.Errorf("invalid json")
	}
	return nil
}

// EditFile represents an edit file command
type EditFile struct {
	NoJson bool `long:"no-json" description:"do not validate JSON"`
}

type Editor interface {
	LaunchTempFile(prefix string, initialContent io.Reader) (edited []byte, path string, err error)
}

func (cmd *EditFile) Execute(args []string) error {
	init_log()
	root := NewDatastore(option.Datadir)
	buf := &bytes.Buffer{}
	if err := root.Read(args[0], buf); err != nil {
		slog.Error("read failed", "name", args[0], "error", err)
		return err
	}
	slog.Info("launch editor", "name", args[0])
	schema := &chkjson{ds: root}
	var edit Editor
	if !cmd.NoJson {
		edit = editor.NewValidatingEditor(schema)
	} else {
		edit = editor.NewEditor()
	}
	old := buf.Bytes()
	olddata := root.ParseJSON(string(old))
	if olddata != nil {
		if b, err := json.MarshalIndent(olddata, "", "  "); err == nil {
			old = b
		}
	}
	edited, path, err := edit.LaunchTempFile(filepath.Base(args[0]), buf)
	defer os.Remove(path)
	if err != nil {
		slog.Error("edit failed", "name", args[0], "error", err)
		if strings.Contains(err.Error(), "no changes made") {
			return ErrNotChanged
		}
		return err
	}
	if bytes.Equal(edited, old) {
		slog.Info("no changes made", "name", args[0])
		return ErrNotChanged
	}
	newdata := root.ParseJSON(string(edited))
	if olddata != nil && newdata != nil && reflect.DeepEqual(olddata, newdata) {
		slog.Info("no changes in data", "name", args[0])
		return ErrNotChanged
	}
	slog.Info("change", "name", args[0], "before", string(old), "after", string(edited))
	return root.Write(args[0], bytes.NewReader(edited), []byte{}, "")
}
