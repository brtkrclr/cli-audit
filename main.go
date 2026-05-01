package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const helpText = `cli-audit: A CLI tool to audit installed applications

Usage:
  cli-audit [flags]
  cli-audit search <query>

Flags:
  -h, --help        Show this help message
  -s, --search      Search for a specific CLI tool directly without opening the interactive UI

Description:
  cli-audit lists all installed CLI apps via brew, npm, and apt.
  It maps binaries to their source packages, resolves shell aliases,
  and parses shell history to show you how often and when you last
  used these tools. This helps identify unused packages that can be removed.
`

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

type AppInfo struct {
	Manager    string
	Name       string
	Version    string
	Time       time.Time
	Binaries   []string
	UsageCount int
	LastUsed   time.Time
	SizeBytes  int64
	SizeString string
}

type CommandStats struct {
	Count    int
	LastUsed time.Time
}

var searchDirect string

func init() {
	flag.StringVar(&searchDirect, "s", "", "Search for a specific CLI tool directly")
	flag.StringVar(&searchDirect, "search", "", "Search for a specific CLI tool directly")
	flag.Usage = func() {
		fmt.Print(helpText)
	}
}

func main() {
	flag.Parse()

	if len(flag.Args()) > 0 && flag.Arg(0) == "help" {
		fmt.Print(helpText)
		os.Exit(0)
	}

	if len(flag.Args()) > 0 && flag.Arg(0) == "search" && len(flag.Args()) > 1 {
		searchDirect = flag.Arg(1)
	}

	apps, err := getInstalledApps()
	if err != nil {
		fmt.Printf("Error collecting apps: %v\n", err)
		os.Exit(1)
	}

	// CLI Direct Search
	if searchDirect != "" {
		search := strings.ToLower(searchDirect)
		fmt.Printf("%-10s | %-25s | %-12s | %-25s | %-15s | %-6s | %-10s | %-15s\n", "MANAGER", "NAME", "VERSION", "BINARIES", "INSTALLED", "USAGE", "SIZE", "LAST USED")
		fmt.Println(strings.Repeat("-", 135))
		for _, app := range apps {
			if strings.Contains(strings.ToLower(app.Name), search) ||
				strings.Contains(strings.ToLower(strings.Join(app.Binaries, " ")), search) {

				timeStr := "Unknown"
				if !app.Time.IsZero() {
					timeStr = app.Time.Format("02 Jan 06")
				}
				lastUsedStr := "Never"
				if !app.LastUsed.IsZero() {
					lastUsedStr = app.LastUsed.Format("02 Jan 06")
				}

				binStr := strings.Join(app.Binaries, ", ")
				if len(binStr) > 25 {
					binStr = binStr[:22] + "..."
				}

				fmt.Printf("%-10s | %-25s | %-12s | %-25s | %-15s | %-6d | %-10s | %-15s\n",
					app.Manager, app.Name, app.Version, binStr, timeStr, app.UsageCount, app.SizeString, lastUsedStr)
			}
		}
		os.Exit(0)
	}

	// Interactive UI
	t := table.New(
		table.WithFocused(true),
		table.WithHeight(25),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	m := model{
		table:   t,
		apps:    apps,
		filter:  "",
		width:   100, // default width
		height:  30,  // default height
		sortCol: SortLastUsed,
	}
	m.updateTable()

	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}

const (
	SortLastUsed = iota
	SortUsage
	SortSize
	SortName
)

type model struct {
	table   table.Model
	apps    []AppInfo
	filter  string
	width   int
	height  int
	sortCol int
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			return m, tea.Quit
		case "backspace", "delete":
			if len(m.filter) > 0 {
				m.filter = m.filter[:len(m.filter)-1]
				m.updateTable()
			}
		case "tab":
			m.sortCol = (m.sortCol + 1) % 4
			m.updateTable()
		default:
			// allow typing characters for search
			if len(msg.String()) == 1 && msg.String() >= " " && msg.String() <= "~" {
				m.filter += msg.String()
				m.updateTable()
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateTable()
	}
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *model) updateTable() {
	filtered := []AppInfo{}
	search := strings.ToLower(m.filter)
	for _, app := range m.apps {
		if search == "" || strings.Contains(strings.ToLower(app.Name), search) ||
			strings.Contains(strings.ToLower(strings.Join(app.Binaries, " ")), search) {
			filtered = append(filtered, app)
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		switch m.sortCol {
		case SortUsage:
			if filtered[i].UsageCount == filtered[j].UsageCount {
				return filtered[i].Name < filtered[j].Name
			}
			return filtered[i].UsageCount > filtered[j].UsageCount
		case SortSize:
			if filtered[i].SizeBytes == filtered[j].SizeBytes {
				return filtered[i].Name < filtered[j].Name
			}
			return filtered[i].SizeBytes > filtered[j].SizeBytes
		case SortName:
			return filtered[i].Name < filtered[j].Name
		default: // SortLastUsed
			if filtered[i].LastUsed.Equal(filtered[j].LastUsed) {
				return filtered[i].Time.After(filtered[j].Time)
			}
			return filtered[i].LastUsed.After(filtered[j].LastUsed)
		}
	})

	availableWidth := m.width - 4
	if availableWidth < 50 {
		availableWidth = 50
	}

	cols := []table.Column{
		{Title: "Manager", Width: availableWidth * 8 / 100},
		{Title: "Name", Width: availableWidth * 20 / 100},
		{Title: "Version", Width: availableWidth * 12 / 100},
		{Title: "Binaries", Width: availableWidth * 18 / 100},
		{Title: "Installed", Width: availableWidth * 12 / 100},
		{Title: "Usage", Width: availableWidth * 8 / 100},
		{Title: "Size", Width: availableWidth * 8 / 100},
		{Title: "Last Used", Width: availableWidth * 12 / 100},
	}
	m.table.SetColumns(cols)
	m.table.SetWidth(m.width)
	m.table.SetHeight(m.height - 6)

	var rows []table.Row
	for _, app := range filtered {
		timeStr := "Unknown"
		if !app.Time.IsZero() {
			timeStr = app.Time.Format("02 Jan 06")
		}

		lastUsedStr := "Never"
		if !app.LastUsed.IsZero() {
			lastUsedStr = app.LastUsed.Format("02 Jan 06")
		}

		binStr := strings.Join(app.Binaries, ", ")
		maxBinLen := cols[3].Width - 2
		if maxBinLen > 3 && len(binStr) > maxBinLen {
			binStr = binStr[:maxBinLen-3] + "..."
		}

		rows = append(rows, table.Row{
			app.Manager,
			app.Name,
			app.Version,
			binStr,
			timeStr,
			fmt.Sprintf("%d", app.UsageCount),
			app.SizeString,
			lastUsedStr,
		})
	}
	m.table.SetRows(rows)
}

func (m model) View() string {
	searchStr := ""
	if m.filter != "" {
		searchStr = fmt.Sprintf("Search: %s", m.filter)
	} else {
		searchStr = "Type to filter..."
	}

	sortStr := "Last Used"
	switch m.sortCol {
	case SortUsage:
		sortStr = "Usage Count"
	case SortSize:
		sortStr = "Size"
	case SortName:
		sortStr = "Name"
	}

	helpText := fmt.Sprintf("\n  %s | Sort: %s (press tab to change)\n  Press esc to quit.\n", 
		lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render(searchStr),
		lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Render(sortStr))

	return baseStyle.Render(m.table.View()) + helpText
}

func getInstalledApps() ([]AppInfo, error) {
	stats := parseHistory()

	var apps []AppInfo

	// Brew
	brewApps, err := getBrewAppsFast(stats)
	if err == nil {
		apps = append(apps, brewApps...)
	}

	// NPM
	npmApps, err := getNpmAppsFast(stats)
	if err == nil {
		apps = append(apps, npmApps...)
	}

	// APT (Linux)
	aptApps, err := getAptApps(stats)
	if err == nil {
		apps = append(apps, aptApps...)
	}

	return apps, nil
}
