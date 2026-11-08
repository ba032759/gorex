package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"

	"gorex/internal/rex"
)

// settings are the user's choices, kept in settings.json beside the
// server's state.
type settings struct {
	// Appearance is "light", "dark", or "" to follow the system.
	Appearance       string  `json:"appearance,omitempty"`
	FontSize         float32 `json:"fontSize,omitempty"`
	MacOSKeybindings *bool   `json:"macosKeybindings,omitempty"`
}

// macosKeybindings tells to take the macOS shortcuts with Control or
// Super for Command, the keymap's "macos" preset, for keyboards remapped to
// work as a Mac's: Control and a letter are then GoRex's, not the
// terminal's, so it is off unless chosen.
func (s settings) macosKeybindings() bool {
	return s.MacOSKeybindings != nil && *s.MacOSKeybindings
}

const defaultFontSize = 11.6

var prefs settings

func settingsPath() string { return filepath.Join(rex.Dir(), "settings.json") }

// loadSettingsQuiet reads the settings, for the CLI: the keymap's preset
// follows them.
func loadSettingsQuiet() {
	if b, err := os.ReadFile(settingsPath()); err == nil {
		_ = json.Unmarshal(b, &prefs) // the app reports bad settings; the CLI keeps the defaults
	}
}

func loadSettings() {
	loadSettingsQuiet()
	if prefs.FontSize < 6 || prefs.FontSize > 40 {
		prefs.FontSize = defaultFontSize
	}
	termFont.Size = prefs.FontSize
	applyAppearance()
	applyKeybindingLabels()
}

func saveSettings() {
	b, _ := json.MarshalIndent(prefs, "", "  ")
	os.MkdirAll(rex.Dir(), 0o700)
	if err := os.WriteFile(settingsPath(), b, 0o600); err != nil {
		log.Printf("saving the settings: %v", err)
	}
}

func applyAppearance() {
	switch prefs.Appearance {
	case "light":
		mygo.Theme.SetSource(mygo.ThemeLight)
	case "dark":
		mygo.Theme.SetSource(mygo.ThemeDark)
	default:
		mygo.Theme.SetSource(mygo.ThemeSystem)
	}
}

func (a *App) setAppearance(v string) {
	prefs.Appearance = v
	applyAppearance()
	saveSettings()
}

// setFontSize changes the size of the terminals' text, in every pane.
func (a *App) setFontSize(size float32) {
	size = min(max(size, 8), 32)
	prefs.FontSize = size
	termFont.Size = size
	for _, t := range a.tabs {
		for _, p := range t.panes() {
			if p.term != nil {
				p.term.SetFont(termFont)
			}
		}
	}
	saveSettings()
}
