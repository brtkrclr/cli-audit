package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseAliases(t *testing.T) {
	tempDir := t.TempDir()

	bashAliases := `
alias ll='ls -la'
alias g='git'
# alias ignored='should not parse'
alias complex="docker-compose up -d"
`
	err := os.WriteFile(filepath.Join(tempDir, ".bash_aliases"), []byte(bashAliases), 0644)
	if err != nil {
		t.Fatalf("Failed to write mock aliases: %v", err)
	}

	aliases := parseAliases(tempDir)

	expected := map[string]string{
		"ll":      "ls",
		"g":       "git",
		"complex": "docker-compose",
	}

	for k, v := range expected {
		if aliases[k] != v {
			t.Errorf("Expected alias %s to map to %s, but got %s", k, v, aliases[k])
		}
	}

	if _, ok := aliases["ignored"]; ok {
		t.Errorf("Commented alias was incorrectly parsed")
	}
}

func TestProcessCmdLine(t *testing.T) {
	tests := []struct {
		name          string
		cmdLine       string
		aliases       map[string]string
		existingStats map[string]CommandStats
		expectedCmd   string
		expectedCount int
	}{
		{
			name:          "Simple command",
			cmdLine:       "ls -la /tmp",
			aliases:       nil,
			existingStats: make(map[string]CommandStats),
			expectedCmd:   "ls",
			expectedCount: 1,
		},
		{
			name:          "Sudo command",
			cmdLine:       "sudo apt-get update",
			aliases:       nil,
			existingStats: make(map[string]CommandStats),
			expectedCmd:   "apt-get",
			expectedCount: 1,
		},
		{
			name:          "Empty command",
			cmdLine:       "   ",
			aliases:       nil,
			existingStats: make(map[string]CommandStats),
			expectedCmd:   "", // won't be recorded
			expectedCount: 0,
		},
		{
			name:    "Aliased command",
			cmdLine: "g co main",
			aliases: map[string]string{
				"g": "git",
			},
			existingStats: make(map[string]CommandStats),
			expectedCmd:   "git",
			expectedCount: 1,
		},
		{
			name:          "Path command",
			cmdLine:       "/usr/local/bin/python script.py",
			aliases:       nil,
			existingStats: make(map[string]CommandStats),
			expectedCmd:   "python",
			expectedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now()
			stats := tt.existingStats
			processCmdLine(tt.cmdLine, now, stats, tt.aliases)

			if tt.expectedCmd != "" {
				if st, ok := stats[tt.expectedCmd]; !ok {
					t.Errorf("Expected command %s to be recorded in stats", tt.expectedCmd)
				} else {
					if st.Count != tt.expectedCount {
						t.Errorf("Expected count %d for %s, got %d", tt.expectedCount, tt.expectedCmd, st.Count)
					}
					if !st.LastUsed.Equal(now) {
						t.Errorf("Expected last used time to be updated")
					}
				}
			} else {
				if len(stats) > 0 {
					t.Errorf("Expected no commands to be recorded, but got %d", len(stats))
				}
			}
		})
	}
}
