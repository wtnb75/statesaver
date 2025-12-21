package main

import (
	"log/slog"
	"os"

	"github.com/jessevdk/go-flags"
)

var option struct {
	Verbose bool   `short:"v" long:"verbose" description:"DEBUG level"`
	Quiet   bool   `short:"q" long:"quiet" description:"WARNING level"`
	Datadir string `short:"d" long:"data-dir" required:"true" env:"STSV_DATADIR" description:"data directory to store state"`
}

func init_log() {
	var level = slog.LevelInfo
	if option.Verbose {
		level = slog.LevelDebug
	} else if option.Quiet {
		level = slog.LevelWarn
	}
	slog.SetLogLoggerLevel(level)
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
}

type SubCommand struct {
	Name  string
	Short string
	Long  string
	Data  interface{}
}

func realMain() int {
	var err error
	commands := []SubCommand{
		{Name: "server", Short: "boot webserver", Long: "boot webserver", Data: &WebServer{}},
		{Name: "ls", Short: "list files", Long: "list state files", Data: &LsTree{}},
		{Name: "cat", Short: "cat files", Long: "cat file contents", Data: &Cat{}},
		{Name: "put", Short: "put files", Long: "put file contents", Data: &Put{}},
		{Name: "history", Short: "list history", Long: "list history of files", Data: &History{}},
		{Name: "hcat", Short: "cat history", Long: "cat history of files", Data: &HistoryCat{}},
		{Name: "prune", Short: "prune history", Long: "remove old history", Data: &Prune{}},
		{Name: "rollback", Short: "rollback to history", Long: "rollback to history", Data: &HistoryRollback{}},
	}
	parser := flags.NewParser(&option, flags.Default)
	for _, cmd := range commands {
		_, err = parser.AddCommand(cmd.Name, cmd.Short, cmd.Long, cmd.Data)
		if err != nil {
			slog.Error(cmd.Name, "error", err)
			return -1
		}
	}
	if _, err := parser.Parse(); err != nil {
		init_log()
		if _, ok := err.(*flags.Error); ok {
			return 0
		}
		slog.Error("error exit", "error", err)
		parser.WriteHelp(os.Stdout)
		return 1
	}
	return 0
}

func main() {
	os.Exit(realMain())
}
