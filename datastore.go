package main

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"time"

	"github.com/spf13/afero"
)

// DsIf is the interface for datastore operations
type DsIf interface {
	Read(name string, out io.Writer) error
	Delete(name string) error
	Write(name string, input io.Reader, hash []byte, lockid string) error
	Lock(name string, lockinfo string) error
	Unlock(name string, lockinfo string) error
	Walk(fn func(e FileEntry) error) error
	History(path string) []FileEntry
	ReadHistory(name string, history string) (io.ReadCloser, error)
}

// Datastore implements DsIf using the afero.BasePathFs
type Datastore struct {
	DsIf
	RootDir  *afero.BasePathFs
	RootName string
}

// NewDatastore creates a new Datastore rooted at the given directory
func NewDatastore(root string) Datastore {
	bpfs := afero.NewBasePathFs(afero.NewOsFs(), root)
	return Datastore{
		RootDir:  bpfs.(*afero.BasePathFs),
		RootName: root,
	}
}

// ParseJSON parses a JSON string into a map
func (d *Datastore) ParseJSON(data string) map[string]interface{} {
	res := make(map[string]interface{})
	if err := json.Unmarshal([]byte(data), &res); err != nil {
		slog.Error("json parse error", "error", err)
		return nil
	}
	return res
}

// File constructs a file path within the datastore
func (d *Datastore) File(name ...string) (string, error) {
	slog.Debug("find file", "name", name)
	path := filepath.Join(name...)
	ret, err := d.RootDir.RealPath(path)
	if err != nil {
		return ret, err
	}
	slog.Debug("rel", "ret", ret, "root", d.RootDir.Name())
	return filepath.Rel(d.RootName, ret)
}

// Tempstr generates a temporary string for file naming
func (d *Datastore) Tempstr(name string) string {
	return strconv.FormatInt(time.Now().UnixNano(), 32)
}

// set_current sets the 'current' symlink to point to the target file
func (d *Datastore) set_current(name string, target string) error {
	if linkto, err := d.File(name, "current"); err != nil {
		slog.Error("invalid filename?", "name", name, "error", err)
		return ErrInvalidPath
	} else {
		slog.Debug("check exists", "linkto", linkto)
		if _, err := d.RootDir.Stat(linkto); err == nil {
			slog.Debug("removing old", "linkto", linkto)
			if err := d.RootDir.Remove(linkto); err != nil {
				slog.Error("remove current", "name", linkto, "erroo", err)
				return err
			}
		}
		slog.Debug("creating symlink", "newname", target, "linkto", linkto)
		// d.RootDir.SymlinkIfPossible(newname, linkto)
		if realto, err := d.RootDir.RealPath(linkto); err != nil {
			slog.Error("realto", "error", err, "linkto", linkto)
			return err
		} else {
			if err = os.Symlink(target, realto); err != nil {
				slog.Error("symlink", "error", err, "newname", target, "realto", realto)
				return err
			}
		}
	}
	return nil
}

// Write writes data to a file in the datastore
func (d *Datastore) Write(name string, input io.Reader, hash []byte, lockid string) error {
	slog.Debug("write", "name", name, "hash", fmt.Sprintf("%x", hash), "lockid", lockid)
	newname, err := d.File(name, d.Tempstr(name))
	if err != nil {
		slog.Error("invalid filename?", "name", name, "error", err)
		return ErrInvalidPath
	}
	if lockid != "" {
		if d.LockCheck(name, lockid) != nil {
			return ErrLocked
		}
	}
	parent := filepath.Dir(newname)
	if err := d.RootDir.MkdirAll(parent, 0o755); err != nil {
		slog.Error("mkdir", "name", name, "error", err)
		return err
	}
	var input2 io.Reader
	hashfp := md5.New()
	if len(hash) != 0 {
		input2 = io.TeeReader(input, hashfp)
	} else {
		input2 = input
	}
	if err := afero.WriteReader(d.RootDir, newname, input2); err != nil {
		slog.Error("write", "error", err, "name", newname)
	}
	if len(hash) != 0 {
		hashb := hashfp.Sum(nil)
		if len(hash) != 0 && !reflect.DeepEqual(hash, hashb) {
			slog.Error("hash mismatch", "name", name)
			if err := d.RootDir.Remove(newname); err != nil {
				slog.Error("cannot unlink invalid file", "name", newname, "error", err)
			}
			return ErrInvalidHash
		}
	}
	return d.set_current(name, filepath.Base(newname))
}

// Read reads data from a file in the datastore
func (d *Datastore) Read(name string, out io.Writer) error {
	slog.Debug("read", "name", name)
	path, err := d.File(name, "current")
	if err != nil {
		slog.Error("invalid filename?", "name", name, "error", err)
		return ErrInvalidPath
	}
	if fp, err := d.RootDir.Open(path); err != nil {
		slog.Error("open file", "error", err, "name", name)
		return ErrNotFound
	} else {
		defer fp.Close()
		written, err := io.Copy(out, fp)
		if err != nil {
			slog.Error("partial read", "written", written, "name", name)
		}
	}
	return nil
}

// Delete removes a file from the datastore
func (d *Datastore) Delete(name string) error {
	slog.Debug("delete", "name", name)
	path, err := d.File(name, "current")
	if err != nil {
		slog.Error("invalid filename?", "name", name, "error", err)
		return ErrInvalidPath
	}
	if err = d.RootDir.Remove(path); err != nil {
		slog.Error("unlink error", "name", name, "error", err)
		return err
	}
	return nil
}

// Lock locks a file in the datastore
func (d *Datastore) Lock(name string, lockinfo string) error {
	slog.Debug("lock", "name", name, "lockinfo", lockinfo)
	path, err := d.File(name, "lock")
	if err != nil {
		slog.Error("invalid filename?", "name", name, "error", err)
		return err
	}
	if fi, err := d.RootDir.Stat(path); err == nil {
		slog.Warn("lock exists", "name", name, "error", err, "fi", fi)
		return ErrLocked
	}
	if err := d.RootDir.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		slog.Error("mkdir failed", "path", path, "error", err)
		return err
	}
	return afero.WriteFile(d.RootDir, path, []byte(lockinfo), 0o644)
}

// LockRead reads the lock information for a file
func (d *Datastore) LockRead(name string) (string, error) {
	slog.Debug("lock-read", "name", name)
	path, err := d.File(name, "lock")
	if err != nil {
		slog.Error("invalid filename?", "name", name, "error", err)
		return "", err
	}
	content, err := afero.ReadFile(d.RootDir, path)
	if err != nil {
		slog.Info("cannot read lock", "name", name)
		return "", ErrUnlocked
	}
	return string(content), nil
}

// LockCheck checks if the provided lock ID matches the stored lock
func (d *Datastore) LockCheck(name string, lockid string) error {
	slog.Debug("cheking lock")
	if lockstr, err := d.LockRead(name); err == nil {
		lockdata := d.ParseJSON(lockstr)
		slog.Debug("check lock id", "lockdata", lockdata, "lockid", lockid)
		if lockdata["ID"] != lockid {
			return ErrLocked
		}
	}
	return nil
}

// Unlock unlocks a file in the datastore
func (d *Datastore) Unlock(name string, lockinfo string) error {
	slog.Debug("unlock", "name", name, "lockinfo", lockinfo)
	path, err := d.File(name, "lock")
	if err != nil {
		slog.Error("invalid filename?", "name", name, "error", err)
		return err
	}
	match_data := d.ParseJSON(lockinfo)
	if match_data != nil {
		content, err := afero.ReadFile(d.RootDir, path)
		if err != nil {
			slog.Error("cannot read lock", "name", name)
			return ErrUnlocked
		}
		prev_data := d.ParseJSON(string(content))
		if match_data["ID"].(string) != prev_data["ID"].(string) {
			return ErrLocked
		}
	}
	if err = d.RootDir.Remove(path); err != nil {
		slog.Error("cannot remove link", "name", name)
		return err
	}
	return nil
}

// FileEntry represents a file entry in the datastore
type FileEntry struct {
	Name      string
	Locked    bool
	Timestamp time.Time
	Size      int64
}

// Walk walks through all files in the datastore and applies the given function
func (d *Datastore) Walk(fn func(e FileEntry) error) error {
	slog.Debug("walk", "root", d.RootName)
	return afero.Walk(d.RootDir, "/", func(path string, info fs.FileInfo, err error) error {
		slog.Debug("walk-cb", "path", path, "info", info, "error", err)
		if err != nil {
			slog.Error("walkdir", "error", err, "path", path)
			return err
		}
		if info.Name() == "current" && (info.Mode().Type()&os.ModeSymlink == os.ModeSymlink) {
			slog.Debug("current", "path", path, "info", info)
			fi, err := d.RootDir.Stat(path)
			if err != nil {
				slog.Warn("current not found", "path", path, "info", info)
				return err
			}
			lockfn := filepath.Join(path, "..", "lock")
			locked := false
			slog.Debug("check lock", "path", path, "lockfile", lockfn)
			_, err = d.RootDir.Stat(lockfn)
			if err == nil {
				slog.Warn("lock exists", "path", path, "lockfile", lockfn)
				locked = true
			}
			if fn(FileEntry{
				Name:      filepath.Dir(path),
				Locked:    locked,
				Timestamp: fi.ModTime(),
				Size:      fi.Size(),
			}) != nil {
				return filepath.SkipDir
			}
		}
		return nil
	})
}

// History retrieves the history of a file in the datastore
func (d *Datastore) History(path string) []FileEntry {
	slog.Debug("find history", "path", path)
	res := []FileEntry{}
	cur, err := d.File(path, "current")
	if err != nil {
		slog.Error("current", "error", err, "path", path)
		return res
	}
	slog.Debug("current", "cur", cur, "path", path)
	linkto, err := d.RootDir.ReadlinkIfPossible(cur)
	if err != nil {
		slog.Error("readlink", "error", err, "path", path)
		return res
	}
	dirn, err := d.File(path)
	if err != nil {
		slog.Error("history", "error", err, "path", path)
	} else {
		files, err := afero.ReadDir(d.RootDir, dirn)
		if err != nil {
			slog.Error("readdir", "error", err, "dirn", dirn)
		} else {
			for _, ent := range files {
				if ent.IsDir() || ent.Name() == "lock" || !ent.Mode().IsRegular() {
					continue
				}
				fi, err := d.RootDir.Stat(filepath.Join(dirn, ent.Name()))
				if err != nil {
					slog.Error("info", "path", dirn, "name", ent.Name)
				} else {
					res = append(res, FileEntry{
						Name:      fi.Name(),
						Locked:    linkto == fi.Name(),
						Timestamp: fi.ModTime(),
						Size:      fi.Size(),
					})
				}
			}
		}
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].Timestamp.After(res[j].Timestamp)
	})
	return res
}

// ReadHistory reads a specific version of a file from the datastore
func (d *Datastore) ReadHistory(name string, history string) (io.ReadCloser, error) {
	slog.Debug("reading history", "name", name, "history", history)
	path, err := d.File(name, history)
	if err != nil {
		slog.Error("invalid filename?", "name", name, "error", err)
		return nil, ErrInvalidPath
	}
	return d.RootDir.Open(path)
}

// Rollback rolls back a file to a specific history version
func (d *Datastore) Rollback(name string, history string) error {
	slog.Debug("rollback to history", "name", name, "history", history)
	path, err := d.File(name, history)
	if err != nil {
		slog.Error("invalid filename?", "name", name, "error", err)
		return ErrInvalidPath
	}
	if _, err := d.RootDir.Stat(path); err != nil {
		slog.Error("target not found", "name", name, "error", err)
		return ErrNotFound
	}
	return d.set_current(name, history)
}

// Prune removes old history versions of a file in the datastore
func (d *Datastore) Prune(name string, keep int, dry bool) error {
	ent := d.History(name)
	slog.Debug("prune", "length", len(ent), "names", ent)
	if len(ent) <= keep {
		slog.Debug("nothing to do", "entries", len(ent), "keep", keep)
		return nil
	}
	for _, i := range ent[keep:] {
		if i.Locked {
			slog.Debug("skip current", "name", i.Name)
			continue
		}
		path, err := d.File(name, i.Name)
		if err != nil {
			slog.Error("invalid history name", "name", name, "history", i.Name, "error", err)
			return err
		}
		slog.Info("removing", "name", name, "history", i.Name, "dry", dry, "path", path)
		if !dry {
			if err := d.RootDir.Remove(path); err != nil {
				slog.Error("cannot remove", "name", name, "history", i.Name, "path", path, "error", err)
				return err
			}
		}
	}
	return nil
}
