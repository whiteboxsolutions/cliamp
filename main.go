// Package main is the entry point for the CLIAMP terminal music player.
package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gopxl/beep/v2"

	"winamp-cli/external/navidrome"
	"winamp-cli/player"
	"winamp-cli/playlist"
	"winamp-cli/ui"
)

// audioExts is the set of file extensions the player can decode.
var audioExts = map[string]bool{
	".mp3":  true,
	".wav":  true,
	".flac": true,
	".ogg":  true,
}

func run() error {
	navURL := os.Getenv("NAVIDROME_URL")
	navUser := os.Getenv("NAVIDROME_USER")
	navPass := os.Getenv("NAVIDROME_PASS")

	var navClient *navidrome.NavidromeClient
	if navURL != "" && navUser != "" && navPass != "" {
		navClient = &navidrome.NavidromeClient{URL: navURL, User: navUser, Password: navPass}
	}

	if len(os.Args) < 2 && navClient == nil {
		return errors.New("usage: cliamp <file|folder> [...] or set NAVIDROME_URL, NAVIDROME_USER, NAVIDROME_PASS")
	}

	// Expand shell globs and resolve directories into audio files
	var files []string
	for _, arg := range os.Args[1:] {
		matches, err := filepath.Glob(arg)
		if err != nil || len(matches) == 0 {
			matches = []string{arg}
		}
		for _, path := range matches {
			resolved, err := collectAudioFiles(path)
			if err != nil {
				return fmt.Errorf("scanning %s: %w", path, err)
			}
			files = append(files, resolved...)
		}
	}

	if len(files) == 0 && navClient == nil {
		return errors.New("no playable files found")
	}

	pl := playlist.New()
	for _, f := range files {
		pl.Add(playlist.TrackFromPath(f))
	}

	sr := beep.SampleRate(44100)
	p := player.New(sr)
	defer p.Close()

	// Launch the TUI with the client injected
	m := ui.NewModel(p, pl, navClient)
	prog := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := prog.Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}

	return nil
}

// collectAudioFiles returns audio file paths for the given argument.
// If path is a directory, it walks it recursively collecting supported files.
// If path is a file with a supported extension, it returns it directly.
func collectAudioFiles(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		if audioExts[strings.ToLower(filepath.Ext(path))] {
			return []string{path}, nil
		}
		return nil, nil
	}

	var files []string
	err = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && audioExts[strings.ToLower(filepath.Ext(p))] {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(files)
	return files, nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
