package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

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

							sizeBytes, _ := getDirSize(pkgPath)

							apps = append(apps, AppInfo{
								Manager:    "npm",
								Name:       name,
								Version:    info.Version,
								Time:       installTime,
								Binaries:   bins,
								UsageCount: totalUsage,
								LastUsed:   lastUsed,
								SizeBytes:  sizeBytes,
								SizeString: formatSize(sizeBytes),
							})
						}
					}
				}
			}
		}
	}

	return apps, nil
}
