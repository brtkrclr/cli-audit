package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
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
		fmt.Printf("%-10s | %-25s | %-12s | %-25s | %-15s | %-6s | %-15s\n", "MANAGER", "NAME", "VERSION", "BINARIES", "INSTALLED", "USAGE", "LAST USED")
		fmt.Println(strings.Repeat("-", 120))
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

				fmt.Printf("%-10s | %-25s | %-12s | %-25s | %-15s | %-6d | %-15s\n",
					app.Manager, app.Name, app.Version, binStr, timeStr, app.UsageCount, lastUsedStr)
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
		table:  t,
		apps:   apps,
		filter: "",
		width:  100, // default width
		height: 30,  // default height
	}
	m.updateTable()

	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}

type model struct {
	table  table.Model
	apps   []AppInfo
	filter string
	width  int
	height int
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
		if filtered[i].LastUsed.Equal(filtered[j].LastUsed) {
			return filtered[i].Time.After(filtered[j].Time)
		}
		return filtered[i].LastUsed.After(filtered[j].LastUsed)
	})

	availableWidth := m.width - 4
	if availableWidth < 50 {
		availableWidth = 50
	}

	cols := []table.Column{
		{Title: "Manager", Width: availableWidth * 8 / 100},
		{Title: "Name", Width: availableWidth * 22 / 100},
		{Title: "Version", Width: availableWidth * 12 / 100},
		{Title: "Binaries", Width: availableWidth * 20 / 100},
		{Title: "Installed", Width: availableWidth * 12 / 100},
		{Title: "Usage", Width: availableWidth * 8 / 100},
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

	return baseStyle.Render(m.table.View()) +
		"\n  " + lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render(searchStr) +
		"\n  Press esc to quit.\n"
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

// History parsing logic
func parseHistory() map[string]CommandStats {
	stats := make(map[string]CommandStats)
	home, err := os.UserHomeDir()
	if err != nil {
		return stats
	}

	aliases := parseAliases(home)

	// Parse Zsh
	zshHist := filepath.Join(home, ".zsh_history")
	if data, err := os.ReadFile(zshHist); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, ": ") {
				parts := strings.SplitN(line, ";", 2)
				if len(parts) == 2 {
					meta := parts[0]
					metaParts := strings.SplitN(meta[2:], ":", 2)
					if len(metaParts) == 2 {
						ts, _ := strconv.ParseInt(metaParts[0], 10, 64)
						cmdLine := strings.TrimSpace(parts[1])
						processCmdLine(cmdLine, time.Unix(ts, 0), stats, aliases)
					}
				}
			}
		}
	}

	// Parse Bash
	bashHist := filepath.Join(home, ".bash_history")
	if data, err := os.ReadFile(bashHist); err == nil {
		lines := strings.Split(string(data), "\n")
		var lastTime time.Time
		for _, line := range lines {
			if strings.HasPrefix(line, "#") {
				ts, err := strconv.ParseInt(line[1:], 10, 64)
				if err == nil {
					lastTime = time.Unix(ts, 0)
					continue
				}
			}
			processCmdLine(line, lastTime, stats, aliases)
			lastTime = time.Time{}
		}
	}
	return stats
}

func parseAliases(home string) map[string]string {
	aliases := make(map[string]string)
	rcs := []string{".zshrc", ".bashrc", ".bash_aliases"}
	for _, rc := range rcs {
		if data, err := os.ReadFile(filepath.Join(home, rc)); err == nil {
			lines := strings.Split(string(data), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "alias ") {
					parts := strings.SplitN(line[6:], "=", 2)
					if len(parts) == 2 {
						k := strings.TrimSpace(parts[0])
						v := strings.TrimSpace(parts[1])
						v = strings.Trim(v, `"'`)
						vWords := strings.Fields(v)
						if len(vWords) > 0 {
							aliases[k] = vWords[0]
						}
					}
				}
			}
		}
	}
	return aliases
}

func processCmdLine(cmdLine string, ts time.Time, stats map[string]CommandStats, aliases map[string]string) {
	if cmdLine == "" {
		return
	}
	words := strings.Fields(cmdLine)
	if len(words) == 0 {
		return
	}
	cmd := words[0]
	if cmd == "sudo" && len(words) > 1 {
		cmd = words[1]
	}

	cmd = filepath.Base(cmd)
	if resolved, ok := aliases[cmd]; ok {
		cmd = resolved
	}

	s := stats[cmd]
	s.Count++
	if ts.After(s.LastUsed) {
		s.LastUsed = ts
	}
	stats[cmd] = s
}

// Brew Fast Parsing
func getBrewAppsFast(stats map[string]CommandStats) ([]AppInfo, error) {
	brewPrefixBytes, err := exec.Command("brew", "--prefix").Output()
	if err != nil {
		return nil, err
	}
	brewPrefix := strings.TrimSpace(string(brewPrefixBytes))

	pkgToBins := make(map[string][]string)
	binDir := filepath.Join(brewPrefix, "bin")
	if entries, err := os.ReadDir(binDir); err == nil {
		for _, e := range entries {
			if e.Type()&os.ModeSymlink != 0 {
				target, _ := os.Readlink(filepath.Join(binDir, e.Name()))
				if strings.Contains(target, "../Cellar/") {
					parts := strings.Split(target, "/")
					for i, p := range parts {
						if p == "Cellar" && i+1 < len(parts) {
							pkg := parts[i+1]
							pkgToBins[pkg] = append(pkgToBins[pkg], e.Name())
							break
						}
					}
				} else if strings.Contains(target, "../opt/") {
					parts := strings.Split(target, "/")
					for i, p := range parts {
						if p == "opt" && i+1 < len(parts) {
							pkg := parts[i+1]
							pkgToBins[pkg] = append(pkgToBins[pkg], e.Name())
							break
						}
					}
				}
			}
		}
	}

	var apps []AppInfo
	cellarDir := filepath.Join(brewPrefix, "Cellar")
	if pkgs, err := os.ReadDir(cellarDir); err == nil {
		for _, pkg := range pkgs {
			if !pkg.IsDir() {
				continue
			}
			if versions, err := os.ReadDir(filepath.Join(cellarDir, pkg.Name())); err == nil && len(versions) > 0 {
				var latestVer string
				for _, v := range versions {
					if v.IsDir() {
						latestVer = v.Name()
					}
				}
				if latestVer == "" {
					continue
				}

				receiptFile := filepath.Join(cellarDir, pkg.Name(), latestVer, "INSTALL_RECEIPT.json")
				var installTime time.Time
				if data, err := os.ReadFile(receiptFile); err == nil {
					var receipt struct {
						Time int64 `json:"time"`
					}
					if json.Unmarshal(data, &receipt) == nil && receipt.Time > 0 {
						installTime = time.Unix(receipt.Time, 0)
					}
				}

				if installTime.IsZero() {
					if stat, err := os.Stat(filepath.Join(cellarDir, pkg.Name(), latestVer)); err == nil {
						installTime = stat.ModTime()
					}
				}

				bins := pkgToBins[pkg.Name()]
				totalUsage := 0
				var lastUsed time.Time
				for _, b := range bins {
					st := stats[b]
					totalUsage += st.Count
					if st.LastUsed.After(lastUsed) {
						lastUsed = st.LastUsed
					}
				}

				if len(bins) == 0 {
					st := stats[pkg.Name()]
					totalUsage += st.Count
					if st.LastUsed.After(lastUsed) {
						lastUsed = st.LastUsed
					}
				}

				apps = append(apps, AppInfo{
					Manager:    "brew",
					Name:       pkg.Name(),
					Version:    latestVer,
					Time:       installTime,
					Binaries:   bins,
					UsageCount: totalUsage,
					LastUsed:   lastUsed,
				})
			}
		}
	}

	return apps, nil
}

// NPM Fast Parsing
func getNpmAppsFast(stats map[string]CommandStats) ([]AppInfo, error) {
	rootOut, err := exec.Command("npm", "root", "-g").Output()
	if err != nil {
		return nil, err
	}
	npmRoot := strings.TrimSpace(string(rootOut))

	prefixOut, _ := exec.Command("npm", "config", "get", "prefix").Output()
	npmPrefix := strings.TrimSpace(string(prefixOut))

	pkgToBins := make(map[string][]string)
	if npmPrefix != "" {
		binDir := filepath.Join(npmPrefix, "bin")
		if entries, err := os.ReadDir(binDir); err == nil {
			for _, e := range entries {
				if e.Type()&os.ModeSymlink != 0 {
					target, _ := os.Readlink(filepath.Join(binDir, e.Name()))
					if strings.Contains(target, "node_modules/") {
						idx := strings.Index(target, "node_modules/")
						rest := target[idx+len("node_modules/"):]
						parts := strings.Split(rest, "/")
						if len(parts) > 0 {
							pkg := parts[0]
							if strings.HasPrefix(pkg, "@") && len(parts) > 1 {
								pkg = pkg + "/" + parts[1]
							}
							pkgToBins[pkg] = append(pkgToBins[pkg], e.Name())
						}
					}
				}
			}
		}
	}

	var apps []AppInfo
	if entries, err := os.ReadDir(npmRoot); err == nil {
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				pkgs := []string{e.Name()}
				if strings.HasPrefix(e.Name(), "@") {
					if subEntries, err := os.ReadDir(filepath.Join(npmRoot, e.Name())); err == nil {
						pkgs = []string{}
						for _, se := range subEntries {
							if se.IsDir() {
								pkgs = append(pkgs, filepath.Join(e.Name(), se.Name()))
							}
						}
					}
				}

				for _, pkg := range pkgs {
					pkgPath := filepath.Join(npmRoot, pkg)
					pkgJsonPath := filepath.Join(pkgPath, "package.json")
					if data, err := os.ReadFile(pkgJsonPath); err == nil {
						var info struct {
							Name    string `json:"name"`
							Version string `json:"version"`
						}
						if err := json.Unmarshal(data, &info); err == nil {
							name := info.Name
							if name == "" {
								name = strings.ReplaceAll(pkg, string(filepath.Separator), "/")
							}
							bins := pkgToBins[name]
							totalUsage := 0
							var lastUsed time.Time
							for _, b := range bins {
								st := stats[b]
								totalUsage += st.Count
								if st.LastUsed.After(lastUsed) {
									lastUsed = st.LastUsed
								}
							}

							var installTime time.Time
							if stat, err := os.Stat(pkgPath); err == nil {
								installTime = stat.ModTime()
							}

							apps = append(apps, AppInfo{
								Manager:    "npm",
								Name:       name,
								Version:    info.Version,
								Time:       installTime,
								Binaries:   bins,
								UsageCount: totalUsage,
								LastUsed:   lastUsed,
							})
						}
					}
				}
			}
		}
	}

	return apps, nil
}

// APT / dpkg parsing logic (Linux support)
func getAptApps(stats map[string]CommandStats) ([]AppInfo, error) {
	if _, err := exec.LookPath("dpkg-query"); err != nil {
		return nil, fmt.Errorf("dpkg-query not found")
	}

	cmd := exec.Command("dpkg-query", "-W", "-f=${Package}|${Version}|${Status}\n")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	pkgToBins := make(map[string][]string)
	infoDir := "/var/lib/dpkg/info"
	if entries, err := os.ReadDir(infoDir); err == nil {
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".list") {
				pkg := strings.TrimSuffix(e.Name(), ".list")
				pkg = strings.Split(pkg, ":")[0]

				if data, err := os.ReadFile(filepath.Join(infoDir, e.Name())); err == nil {
					lines := strings.Split(string(data), "\n")
					for _, line := range lines {
						if strings.HasPrefix(line, "/usr/bin/") || strings.HasPrefix(line, "/bin/") || strings.HasPrefix(line, "/usr/sbin/") {
							bin := filepath.Base(line)
							pkgToBins[pkg] = append(pkgToBins[pkg], bin)
						}
					}
				}
			}
		}
	}

	var apps []AppInfo
	lines := strings.Split(out.String(), "\n")
	for _, line := range lines {
		parts := strings.Split(line, "|")
		if len(parts) == 3 && strings.Contains(parts[2], "installed") {
			name := parts[0]
			version := parts[1]
			nameBase := strings.Split(name, ":")[0]

			bins := pkgToBins[nameBase]

			// For apt, there are thousands of libs. Only include packages that expose at least one binary.
			if len(bins) == 0 {
				continue
			}

			totalUsage := 0
			var lastUsed time.Time
			for _, b := range bins {
				st := stats[b]
				totalUsage += st.Count
				if st.LastUsed.After(lastUsed) {
					lastUsed = st.LastUsed
				}
			}

			var installTime time.Time
			if stat, err := os.Stat(filepath.Join(infoDir, nameBase+".list")); err == nil {
				installTime = stat.ModTime()
			} else if stat, err := os.Stat(filepath.Join(infoDir, name+".list")); err == nil {
				installTime = stat.ModTime()
			}

			apps = append(apps, AppInfo{
				Manager:    "apt",
				Name:       nameBase,
				Version:    version,
				Time:       installTime,
				Binaries:   bins,
				UsageCount: totalUsage,
				LastUsed:   lastUsed,
			})
		}
	}

	return apps, nil
}
