package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

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
