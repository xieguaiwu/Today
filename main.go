package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/xieguaiwu/Today/internal"
)

// Version is the human-readable release of this build.
const Version = "0.2.0"

func main() {
	var (
		configFlag  string
		dataFlag    string
		statusMode  bool
		resetMode   bool
		versionMode bool
	)
	flag.StringVar(&configFlag, "config", "", "path to the config file (default $XDG_CONFIG_HOME/Today/config.json)")
	flag.StringVar(&dataFlag, "data", "", "path to the history file (default $XDG_DATA_HOME/Today/history.json)")
	flag.BoolVar(&statusMode, "status", false, "print today's progress and exit (for shell prompts and status bars)")
	flag.BoolVar(&resetMode, "reset", false, "clear today's checks and exit")
	flag.BoolVar(&versionMode, "version", false, "print version and exit")
	flag.Parse()

	if versionMode {
		fmt.Printf("Today %s\n", Version)
		return
	}

	cfgPath, err := internal.ConfigPath(configFlag)
	if err != nil {
		fail(err)
	}
	cfg, err := internal.LoadConfig(cfgPath)
	if err != nil {
		fail(err)
	}

	dataPath, err := internal.DataPath(dataFlag)
	if err != nil {
		fail(err)
	}
	hist, err := internal.LoadHistory(dataPath, cfg.HistoryDays)
	if err != nil {
		fail(err)
	}
	if hist.Notice != "" {
		fmt.Fprintf(os.Stderr, "Today: %s\n", hist.Notice)
	}

	today := time.Now().Format(internal.DayLayout)

	if resetMode {
		if err := hist.ResetDay(today); err != nil {
			fail(err)
		}
		fmt.Printf("%s reset\n", today)
		return
	}

	if statusMode {
		done := len(hist.CheckedSet(today))
		fmt.Printf("%s %d/%d streak=%d total=%d\n",
			today, done, len(cfg.Items), hist.Streak(time.Now()), hist.CheckedCount())
		return
	}

	p := tea.NewProgram(internal.New(cfg, hist))
	if _, err := p.Run(); err != nil {
		fail(fmt.Errorf("could not start the TUI (needs an interactive terminal): %w", err))
	}
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "Today: %v\n", err)
	os.Exit(1)
}
