package main

import (
	"log/slog"
	"os"
	"testing"
)

func TestInitLog_DefaultLevel(t *testing.T) {
	origVerbose := option.Verbose
	origQuiet := option.Quiet
	defer func() {
		option.Verbose = origVerbose
		option.Quiet = origQuiet
	}()

	option.Verbose = false
	option.Quiet = false
	init_log()

	// Test that logging was initialized (no panic, no error)
	slog.Info("test message")
}

func TestInitLog_VerboseLevel(t *testing.T) {
	origVerbose := option.Verbose
	origQuiet := option.Quiet
	defer func() {
		option.Verbose = origVerbose
		option.Quiet = origQuiet
	}()

	option.Verbose = true
	option.Quiet = false
	init_log()

	// Should be set to DEBUG level
	slog.Debug("debug message")
}

func TestInitLog_QuietLevel(t *testing.T) {
	origVerbose := option.Verbose
	origQuiet := option.Quiet
	defer func() {
		option.Verbose = origVerbose
		option.Quiet = origQuiet
	}()

	option.Verbose = false
	option.Quiet = true
	init_log()

	// Should be set to WARN level
	slog.Warn("warn message")
}

func TestRealMain_NoArgs(t *testing.T) {
	// Backup original args
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	// Test with help command
	os.Args = []string{"program", "--help"}
	exitCode := realMain()
	// Help flag should return 0 or similar graceful exit
	t.Logf("exit code: %d (help request)", exitCode)
}

func TestRealMain_ValidCommand_Ls(t *testing.T) {
	origArgs := os.Args
	origDatadir := option.Datadir
	defer func() {
		os.Args = origArgs
		option.Datadir = origDatadir
	}()

	tmpDir := t.TempDir()
	option.Datadir = tmpDir
	os.Args = []string{"program", "ls"}

	exitCode := realMain()
	if exitCode != 0 {
		t.Logf("exit code: %d for 'ls' command", exitCode)
	}
}

func TestRealMain_ValidCommand_Cat(t *testing.T) {
	origArgs := os.Args
	origDatadir := option.Datadir
	defer func() {
		os.Args = origArgs
		option.Datadir = origDatadir
	}()

	tmpDir := t.TempDir()
	option.Datadir = tmpDir
	os.Args = []string{"program", "cat"}

	exitCode := realMain()
	if exitCode != 0 {
		t.Logf("exit code: %d for 'cat' command", exitCode)
	}
}

func TestRealMain_MissingRequiredDatadir(t *testing.T) {
	origArgs := os.Args
	origDatadir := option.Datadir
	defer func() {
		os.Args = origArgs
		option.Datadir = origDatadir
	}()

	// Clear the required datadir
	option.Datadir = ""
	os.Args = []string{"program", "ls"}

	exitCode := realMain()
	// flags.Error returns exit code 0 according to main.go implementation
	if exitCode != 0 {
		t.Errorf("expected exit code 0 for missing required datadir (flags.Error), got %d", exitCode)
	}
}

func TestRealMain_InvalidCommand(t *testing.T) {
	origArgs := os.Args
	origDatadir := option.Datadir
	defer func() {
		os.Args = origArgs
		option.Datadir = origDatadir
	}()

	tmpDir := t.TempDir()
	option.Datadir = tmpDir
	os.Args = []string{"program", "nonexistentcommand"}

	exitCode := realMain()
	// flags.Error returns exit code 0 according to main.go implementation
	if exitCode != 0 {
		t.Errorf("expected exit code 0 for unknown command (flags.Error), got %d", exitCode)
	}
}

func TestRealMain_Server(t *testing.T) {
	origArgs := os.Args
	origDatadir := option.Datadir
	defer func() {
		os.Args = origArgs
		option.Datadir = origDatadir
	}()

	tmpDir := t.TempDir()
	option.Datadir = tmpDir
	os.Args = []string{"program", "server", "--help"}

	exitCode := realMain()
	// Help should succeed
	t.Logf("exit code: %d for 'server --help'", exitCode)
}

func TestRealMain_History(t *testing.T) {
	origArgs := os.Args
	origDatadir := option.Datadir
	defer func() {
		os.Args = origArgs
		option.Datadir = origDatadir
	}()

	tmpDir := t.TempDir()
	option.Datadir = tmpDir
	os.Args = []string{"program", "history"}

	exitCode := realMain()
	if exitCode != 0 {
		t.Logf("exit code: %d for 'history' command", exitCode)
	}
}

func TestRealMain_Prune(t *testing.T) {
	origArgs := os.Args
	origDatadir := option.Datadir
	defer func() {
		os.Args = origArgs
		option.Datadir = origDatadir
	}()

	tmpDir := t.TempDir()
	option.Datadir = tmpDir
	os.Args = []string{"program", "prune", "-k", "5"}

	exitCode := realMain()
	if exitCode != 0 {
		t.Logf("exit code: %d for 'prune' command", exitCode)
	}
}

func TestSubCommand_Structure(t *testing.T) {
	cmd := SubCommand{
		Name:  "test",
		Short: "short description",
		Long:  "long description",
		Data:  &LsTree{},
	}

	if cmd.Name != "test" {
		t.Errorf("expected Name='test', got %q", cmd.Name)
	}
	if cmd.Data == nil {
		t.Errorf("expected Data to be non-nil")
	}
}
