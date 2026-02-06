package cmd

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"github.com/GainForest/hypercerts-cli/internal/style"
)

const formMaxWidth = 80

// activityFormResult holds the values collected from the activity creation form.
type activityFormResult struct {
	Title           string
	ShortDesc       string
	Description     string
	WorkScope       string
	StartDate       string
	EndDate         string
	ImageURI        string
	AddContributors bool
	AddLocations    bool
	AddRights       bool
}

// activityFormModel is a bubbletea model that embeds a huh form on the left
// and renders a live activity "card" preview on the right as the user types.
type activityFormModel struct {
	form   *huh.Form
	width  int
	result activityFormResult
	lg     *lipgloss.Renderer
	styles activityFormStyles
}

type activityFormStyles struct {
	base       lipgloss.Style
	header     lipgloss.Style
	card       lipgloss.Style
	cardTitle  lipgloss.Style
	cardLabel  lipgloss.Style
	cardValue  lipgloss.Style
	cardDim    lipgloss.Style
	cardAccent lipgloss.Style
	footer     lipgloss.Style
}

func newActivityFormStyles(lg *lipgloss.Renderer) activityFormStyles {
	accent := lipgloss.AdaptiveColor{Light: "#5A56E0", Dark: "#7571F9"}
	green := lipgloss.AdaptiveColor{Light: "#02BA84", Dark: "#02BF87"}
	dim := lipgloss.AdaptiveColor{Light: "#9B9B9B", Dark: "#626262"}

	return activityFormStyles{
		base: lg.NewStyle().Padding(1, 2, 0, 1),
		header: lg.NewStyle().
			Foreground(accent).
			Bold(true).
			Padding(0, 1, 0, 2),
		card: lg.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accent).
			PaddingLeft(2).
			PaddingRight(2).
			PaddingTop(1).
			PaddingBottom(1).
			MarginTop(1),
		cardTitle: lg.NewStyle().
			Bold(true).
			Foreground(accent),
		cardLabel: lg.NewStyle().
			Foreground(green).
			Bold(true),
		cardValue: lg.NewStyle(),
		cardDim: lg.NewStyle().
			Foreground(dim).
			Italic(true),
		cardAccent: lg.NewStyle().
			Foreground(accent),
		footer: lg.NewStyle().
			Foreground(lipgloss.Color("240")),
	}
}

func newActivityFormModel() activityFormModel {
	m := activityFormModel{width: formMaxWidth}
	m.lg = lipgloss.DefaultRenderer()
	m.styles = newActivityFormStyles(m.lg)

	m.form = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Key("title").
				Title("Title").
				Description("Main title for this hypercert").
				CharLimit(256).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("title is required")
					}
					return nil
				}),

			huh.NewInput().
				Key("shortDesc").
				Title("Short description").
				Description("Brief summary of the activity").
				CharLimit(300).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("short description is required")
					}
					return nil
				}),

			huh.NewInput().
				Key("description").
				Title("Description").
				Description("Longer description (optional)").
				CharLimit(3000),

			huh.NewInput().
				Key("workScope").
				Title("Work scope").
				Description("Free-form scope of work (optional)"),
		).Title("Activity Details"),

		huh.NewGroup(
			huh.NewInput().
				Key("startDate").
				Title("Start date").
				Description("YYYY-MM-DD (optional)"),

			huh.NewInput().
				Key("endDate").
				Title("End date").
				Description("YYYY-MM-DD (optional)"),

			huh.NewInput().
				Key("imageURI").
				Title("Image URI").
				Description("URL to hypercert image (optional)"),
		).Title("Dates & Media"),

		huh.NewGroup(
			huh.NewConfirm().
				Key("addContributors").
				Title("Add contributors?").
				Description("People or orgs involved in this activity"),

			huh.NewConfirm().
				Key("addLocations").
				Title("Add locations?").
				Description("Geographic coordinates for this activity"),

			huh.NewConfirm().
				Key("addRights").
				Title("Add rights?").
				Description("License or rights for this claim"),
		).Title("Linked Records"),
	).
		WithTheme(style.Theme()).
		WithWidth(44).
		WithShowHelp(false).
		WithShowErrors(false)

	return m
}

func (m activityFormModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m activityFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		w := msg.Width
		if w > formMaxWidth {
			w = formMaxWidth
		}
		m.width = w - m.styles.base.GetHorizontalFrameSize()
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Interrupt
		}
	}

	var cmds []tea.Cmd

	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
		cmds = append(cmds, cmd)
	}

	if m.form.State == huh.StateCompleted {
		cmds = append(cmds, tea.Quit)
	}

	return m, tea.Batch(cmds...)
}

func (m activityFormModel) View() string {
	s := m.styles

	if m.form.State == huh.StateCompleted {
		return m.renderCompletedCard()
	}

	// Form (left side)
	v := strings.TrimSuffix(m.form.View(), "\n\n")
	form := m.lg.NewStyle().Margin(1, 0).Render(v)

	// Preview card (right side)
	card := m.renderPreviewCard(lipgloss.Height(form))

	// Compute spacing
	formWidth := lipgloss.Width(form)
	cardWidth := lipgloss.Width(card)
	gap := m.width - formWidth - cardWidth
	if gap < 2 {
		gap = 2
	}
	spacer := strings.Repeat(" ", gap)
	body := lipgloss.JoinHorizontal(lipgloss.Top, form, spacer, card)

	// Header
	header := m.appBoundaryView("  New Hypercert Activity")

	// Footer
	errors := m.form.Errors()
	var footer string
	if len(errors) > 0 {
		var errMsgs []string
		for _, err := range errors {
			errMsgs = append(errMsgs, err.Error())
		}
		footer = m.appErrorBoundaryView(strings.Join(errMsgs, "; "))
	} else {
		footer = m.appBoundaryView(m.form.Help().ShortHelpView(m.form.KeyBinds()))
	}

	return s.base.Render(header + "\n" + body + "\n\n" + footer)
}

// renderPreviewCard renders the live activity preview on the right side.
func (m activityFormModel) renderPreviewCard(height int) string {
	s := m.styles
	const cardWidth = 30

	title := m.form.GetString("title")
	shortDesc := m.form.GetString("shortDesc")
	description := m.form.GetString("description")
	workScope := m.form.GetString("workScope")
	startDate := m.form.GetString("startDate")
	endDate := m.form.GetString("endDate")
	imageURI := m.form.GetString("imageURI")

	var lines []string

	// Title
	if title != "" {
		lines = append(lines, s.cardTitle.Width(cardWidth-4).Render(title))
	} else {
		lines = append(lines, s.cardDim.Render("Untitled"))
	}

	// Short description
	if shortDesc != "" {
		lines = append(lines, s.cardValue.Width(cardWidth-4).Render(shortDesc))
	} else {
		lines = append(lines, s.cardDim.Render("No description yet"))
	}

	lines = append(lines, "")

	// Fields
	if description != "" {
		desc := description
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}
		lines = append(lines, s.cardLabel.Render("Description"))
		lines = append(lines, s.cardValue.Width(cardWidth-4).Render(desc))
		lines = append(lines, "")
	}

	if workScope != "" {
		lines = append(lines, s.cardLabel.Render("Scope"))
		lines = append(lines, s.cardAccent.Render(workScope))
		lines = append(lines, "")
	}

	if startDate != "" || endDate != "" {
		lines = append(lines, s.cardLabel.Render("Period"))
		start := startDate
		if start == "" {
			start = "..."
		}
		end := endDate
		if end == "" {
			end = "..."
		}
		lines = append(lines, s.cardValue.Render(start+" - "+end))
		lines = append(lines, "")
	}

	if imageURI != "" {
		lines = append(lines, s.cardLabel.Render("Image"))
		img := imageURI
		if len(img) > cardWidth-4 {
			img = img[:cardWidth-7] + "..."
		}
		lines = append(lines, s.cardDim.Render(img))
		lines = append(lines, "")
	}

	// Linked records summary
	var links []string
	if m.form.GetBool("addContributors") {
		links = append(links, "contributors")
	}
	if m.form.GetBool("addLocations") {
		links = append(links, "locations")
	}
	if m.form.GetBool("addRights") {
		links = append(links, "rights")
	}
	if len(links) > 0 {
		lines = append(lines, s.cardLabel.Render("Will link"))
		lines = append(lines, s.cardAccent.Render(strings.Join(links, ", ")))
	}

	content := strings.Join(lines, "\n")

	return s.card.
		Width(cardWidth).
		Height(height - 2). // account for card border
		Render(content)
}

// renderCompletedCard renders the final summary after the form is submitted.
func (m activityFormModel) renderCompletedCard() string {
	s := m.styles

	title := m.form.GetString("title")
	shortDesc := m.form.GetString("shortDesc")

	var b strings.Builder
	b.WriteString(s.cardTitle.Render(title))
	b.WriteString("\n")
	b.WriteString(s.cardValue.Render(shortDesc))

	return s.card.
		Width(50).
		Padding(1, 2).
		Render(b.String()) + "\n"
}

func (m activityFormModel) appBoundaryView(text string) string {
	accent := lipgloss.AdaptiveColor{Light: "#5A56E0", Dark: "#7571F9"}
	return lipgloss.PlaceHorizontal(
		m.width,
		lipgloss.Left,
		m.styles.header.Render(text),
		lipgloss.WithWhitespaceChars("/"),
		lipgloss.WithWhitespaceForeground(accent),
	)
}

func (m activityFormModel) appErrorBoundaryView(text string) string {
	red := lipgloss.AdaptiveColor{Light: "#FE5F86", Dark: "#FE5F86"}
	errStyle := m.styles.header.Foreground(red)
	return lipgloss.PlaceHorizontal(
		m.width,
		lipgloss.Left,
		errStyle.Render(text),
		lipgloss.WithWhitespaceChars("/"),
		lipgloss.WithWhitespaceForeground(red),
	)
}

// collectResult extracts form values into an activityFormResult.
func (m activityFormModel) collectResult() activityFormResult {
	return activityFormResult{
		Title:           m.form.GetString("title"),
		ShortDesc:       m.form.GetString("shortDesc"),
		Description:     m.form.GetString("description"),
		WorkScope:       m.form.GetString("workScope"),
		StartDate:       m.form.GetString("startDate"),
		EndDate:         m.form.GetString("endDate"),
		ImageURI:        m.form.GetString("imageURI"),
		AddContributors: m.form.GetBool("addContributors"),
		AddLocations:    m.form.GetBool("addLocations"),
		AddRights:       m.form.GetBool("addRights"),
	}
}

// runActivityForm launches the interactive activity creation form with live preview.
// Returns the collected values or an error (including ErrUserAborted on ctrl+c).
func runActivityForm() (activityFormResult, error) {
	m := newActivityFormModel()
	final, err := tea.NewProgram(m).Run()
	if err != nil {
		return activityFormResult{}, err
	}

	fm, ok := final.(activityFormModel)
	if !ok || fm.form.State != huh.StateCompleted {
		return activityFormResult{}, huh.ErrUserAborted
	}

	return fm.collectResult(), nil
}
