package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nateberkopec/ghwatch/internal/app"
	"github.com/nateberkopec/ghwatch/internal/githubclient"
)

func main() {
	var (
		pollInterval time.Duration
		bellEnabled  bool
	)

	flag.DurationVar(&pollInterval, "interval", 10*time.Second, "how often to refresh watched runs")
	flag.BoolVar(&bellEnabled, "bell", true, "ring the terminal bell when a run state changes")
	flag.Parse()

	cfg := app.Config{
		Client:       githubclient.New(""),
		PollInterval: pollInterval,
		BellEnabled:  bellEnabled,
	}

	program := tea.NewProgram(
		app.New(cfg),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
