package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"gitlab-tui/internal/config"
	"gitlab-tui/internal/gitlab"
	"gitlab-tui/internal/tui"
)

func main() {
	// Ensure sample config exists on first run
	if err := config.EnsureDefaultConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not create default config: %v\n", err)
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if len(cfg.Servers) == 0 {
		fmt.Fprintln(os.Stderr, "No servers configured.")
		fmt.Fprintln(os.Stderr, "Edit ~/.config/gitlab-tui/config.json to add a server.")
		os.Exit(1)
	}

	var serverIdx int
	var projectPath string
	var initialMRIID int
	var initialPipelineID int
	var initialJobID int64
	var startupWarn string
	var detectedRemote string

	if len(os.Args) > 1 {
		arg := os.Args[1]
		if arg == "--help" || arg == "-h" || arg == "help" {
			printHelp()
			os.Exit(0)
		}
		if strings.HasPrefix(arg, "-") {
			fmt.Fprintf(os.Stderr, "Unknown flag: %s\n", arg)
			printHelp()
			os.Exit(1)
		}

		var parseErr error
		serverIdx, projectPath, initialMRIID, initialPipelineID, initialJobID, parseErr = config.ParseGitLabURL(cfg, arg)
		if parseErr != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", parseErr)
			os.Exit(1)
		}
	} else {
		// Auto-detect server+project from git remote, or fall back to default server
		var detected *config.DetectedProject
		detected, detectedRemote = config.DetectFromGit(cfg)

		// Pick which server to use initially
		serverIdx = 0
		for i, s := range cfg.Servers {
			if s.Default {
				serverIdx = i
				break
			}
		}

		if detected != nil {
			// Find matching server index
			for i, s := range cfg.Servers {
				if s.Name == detected.ServerName || s.URL == detected.ServerURL {
					serverIdx = i
					break
				}
			}
			projectPath = detected.ProjectPath
		}
	}

	srv := cfg.Servers[serverIdx]

	// Create GitLab client
	client, err := gitlab.NewClient(srv.URL, srv.Token)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating client: %v\n", err)
		os.Exit(1)
	}

	// Optionally pre-fetch the project
	var project *gitlab.ProjectInfo
	if projectPath != "" {
		project, err = client.GetProject(projectPath)
		if err != nil {
			if initialMRIID > 0 || initialPipelineID > 0 || initialJobID > 0 {
				fmt.Fprintf(os.Stderr, "Error: Could not load project %q from server %s: %v\n", projectPath, srv.URL, err)
				os.Exit(1)
			}
			// Don't fatal for auto-detect — user can still pick a project from TUI.
			// But record the warning so the TUI can show it.
			startupWarn = fmt.Sprintf("Auto-detected remote %q → project %q, but could not load it: %v",
				detectedRemote, projectPath, err)
		}
	} else if detectedRemote != "" {
		startupWarn = fmt.Sprintf("Git remote %q found but did not match any configured server", detectedRemote)
	}

	// Build and run the TUI
	m := tui.New(cfg, serverIdx, client, project, startupWarn, initialMRIID, initialPipelineID, initialJobID)

	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println("GitLab TUI is a terminal UI for GitLab.")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  gitlab-tui [flags]")
	fmt.Println("  gitlab-tui [MR / Pipeline / Job URL]")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  -h, --help      Show help information")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  gitlab-tui")
	fmt.Println("  gitlab-tui https://gitlab.com/group/project/-/merge_requests/123")
	fmt.Println("  gitlab-tui https://gitlab.com/group/project/-/pipelines/33780")
	fmt.Println("  gitlab-tui https://gitlab.com/group/project/-/jobs/155933")
}
