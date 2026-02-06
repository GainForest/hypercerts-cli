package style

import "github.com/charmbracelet/huh"

// Theme returns the huh theme used for all interactive UI in the CLI.
// Change this single function to restyle every form, select, and confirm.
func Theme() *huh.Theme {
	return huh.ThemeCharm()
}
