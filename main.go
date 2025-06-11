package main

import (
	"context"
	"errors"
	"os"
	"sync"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/spotdemo4/quick-commit/internal/git"
	"github.com/spotdemo4/quick-commit/internal/llm"
	"github.com/spotdemo4/quick-commit/internal/tui"
	"github.com/spotdemo4/quick-commit/internal/util"
)

var (
	version  = "0.0.2"
	keywords = []string{"feat", "fix", "docs", "style", "perf", "test", "build", "chore"}
)

func main() {
	// Get config
	conf, err := getConfig()
	if err != nil {
		tui.PrintErr("config error: %v", err)
	}

	// Get diff
	diff, err := git.Diff()
	if err != nil {
		tui.PrintErr("git error: %v", err)
		os.Exit(1)
	}

	// Create context
	ctx, cancel := context.WithCancel(context.Background())
	wg := sync.WaitGroup{}

	// Create channels
	inputChan := make(chan string, 10)
	outputChan := make(chan tui.Msg, 10)

	// Create llm
	llm, err := llm.New(ctx, conf.url, conf.model, conf.headers, conf.options, outputChan)
	if err != nil {
		tui.PrintErr("llm error: %v", err)
		os.Exit(1)
	}

	// Create tea
	t := tea.NewProgram(
		tui.New(version, keywords, inputChan, outputChan),
		tea.WithContext(ctx),
		tea.WithAltScreen(),
	)

	// Start tea
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()

		_, err = t.Run()
		if err != nil && !errors.Is(err, tea.ErrProgramKilled) && !errors.Is(err, tea.ErrInterrupted) {
			tui.PrintErr("tui error: %v", err)
		}
	}()

	// Start llm
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()

		// Get keyword selection from user
		keyword, ok := util.Next(ctx, inputChan)
		if !ok {
			return
		}

		// Generate commit message
		commit := ""
	commit:
		for {
			commit, err = llm.GenerateCommit(ctx, diff, keyword)
			if err != nil {
				if !errors.Is(err, context.Canceled) {
					tui.PrintErr("llm error: %v", err)
				}

				return
			}

			// Send commit to user
			outputChan <- tui.Msg{
				Text: commit,
				Type: tui.MsgCommit,
			}

			// Get validation from user
			input, ok := util.Next(ctx, inputChan)
			if !ok {
				return
			}
			if input != "false" {
				break commit
			}
		}

		// Use that commit message
		out, err := git.Commit(commit)
		if err != nil {
			tui.PrintErr("git error: %v", err)
			return
		}
		tui.Print("%s", out)
	}()

	// Wait for both tea & ollama to finish
	wg.Wait()
}
