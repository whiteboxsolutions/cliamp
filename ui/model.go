// Package ui implements the Bubbletea TUI for the CLIAMP terminal music player.
package ui

import (
	"log"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"winamp-cli/external/navidrome"
	"winamp-cli/player"
	"winamp-cli/playlist"
)

type focusArea int

const (
	focusPlaylist focusArea = iota
	focusEQ
	focusSearch
	focusNavidrome
)

type tickMsg time.Time

// Model is the Bubbletea model for the CLIAMP TUI.
type Model struct {
	player    *player.Player
	playlist  *playlist.Playlist
	vis       *Visualizer
	focus     focusArea
	eqCursor  int // selected EQ band (0-9)
	plCursor  int // selected playlist item
	plScroll  int // scroll offset for playlist view
	plVisible int // max visible playlist items
	titleOff  int // scroll offset for long track titles
	err       error
	quitting  bool
	width     int
	height    int

	navClient    *navidrome.NavidromeClient
	navPlaylists []navidrome.NavidromePlaylist
	navCursor    int
	navLoading   bool

	// Search mode state
	searching     bool
	searchQuery   string
	searchResults []int // indices into playlist tracks
	searchCursor  int
	prevFocus     focusArea // focus to restore on cancel
}

// NewModel creates a Model wired to the given player and playlist.
func NewModel(p *player.Player, pl *playlist.Playlist, nc *navidrome.NavidromeClient) Model {
	m := Model{
		player:    p,
		playlist:  pl,
		vis:       NewVisualizer(44100),
		plVisible: 5,
	}
	if nc != nil {
		m.navClient = nc
		m.focus = focusNavidrome
		m.navLoading = true
	}
	return m
}

func fetchPlaylistsCmd(client *navidrome.NavidromeClient) tea.Cmd {
	return func() tea.Msg {
		pls, err := client.GetPlaylists()
		if err != nil {
			return err
		}
		return pls
	}
}

type tracksLoadedMsg []playlist.Track

func fetchTracksCmd(client *navidrome.NavidromeClient, playlistID string) tea.Cmd {
	return func() tea.Msg {
		tracks, err := client.GetPlaylistTracks(playlistID)
		if err != nil {
			return err
		}
		var pts []playlist.Track
		for _, t := range tracks {
			pts = append(pts, playlist.Track{
				Path:   client.StreamURL(t.ID),
				Title:  t.Title,
				Artist: t.Artist,
			})
		}
		return tracksLoadedMsg(pts)
	}
}

// Init starts the tick timer and requests the terminal size.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{tickCmd(), tea.WindowSize()}
	log.Printf("Setting up initial commands: \n %#v", m)
	if m.navClient != nil {
		log.Println("Fetching Navidrome Playlist")
		cmds = append(cmds, fetchPlaylistsCmd(m.navClient))
	}
	return tea.Batch(cmds...)
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*50, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Update handles messages: key presses, ticks, and window resizes.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		cmd := m.handleKey(msg)
		if m.quitting {
			return m, tea.Quit
		}
		return m, cmd

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tickMsg:
		// Check if the current track finished naturally
		if m.player.IsPlaying() && !m.player.IsPaused() && m.player.TrackDone() {
			m.nextTrack()
		}
		m.titleOff++
		return m, tickCmd()

	case []navidrome.NavidromePlaylist:
		m.navPlaylists = msg
		m.navLoading = false
		return m, nil

	case tracksLoadedMsg:
		m.playlist.Add(msg...)
		m.focus = focusPlaylist
		m.navLoading = false
		if m.playlist.Len() > 0 {
			m.playCurrentTrack()
		}
		return m, nil

	case error:
		m.err = msg
		m.navLoading = false
		return m, nil
	}

	return m, nil
}

// nextTrack advances to the next playlist track and starts playing it.
func (m *Model) nextTrack() {
	track, ok := m.playlist.Next()
	if !ok {
		m.player.Stop()
		return
	}
	m.plCursor = m.playlist.Index()
	m.adjustScroll()
	if err := m.player.Play(track.Path); err != nil {
		m.err = err
	}
}

// prevTrack goes to the previous track, or restarts if >3s into the current one.
func (m *Model) prevTrack() {
	if m.player.Position() > 3*time.Second {
		m.player.Seek(-m.player.Position())
		return
	}
	track, ok := m.playlist.Prev()
	if !ok {
		return
	}
	m.plCursor = m.playlist.Index()
	m.adjustScroll()
	if err := m.player.Play(track.Path); err != nil {
		m.err = err
	}
}

// playCurrentTrack starts playing whatever track the playlist cursor points to.
func (m *Model) playCurrentTrack() {
	track, idx := m.playlist.Current()
	if idx < 0 {
		return
	}
	m.titleOff = 0
	if err := m.player.Play(track.Path); err != nil {
		m.err = err
	}
}

// adjustScroll ensures plCursor is visible in the playlist view.
func (m *Model) adjustScroll() {
	if m.plCursor < m.plScroll {
		m.plScroll = m.plCursor
	}
	if m.plCursor >= m.plScroll+m.plVisible {
		m.plScroll = m.plCursor - m.plVisible + 1
	}
}

// updateSearch filters the playlist by the current search query.
func (m *Model) updateSearch() {
	m.searchResults = nil
	m.searchCursor = 0
	if m.searchQuery == "" {
		return
	}
	query := strings.ToLower(m.searchQuery)
	for i, t := range m.playlist.Tracks() {
		if strings.Contains(strings.ToLower(t.DisplayName()), query) {
			m.searchResults = append(m.searchResults, i)
		}
	}
}
