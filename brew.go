package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

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

				pkgPath := filepath.Join(cellarDir, pkg.Name(), latestVer)
				sizeBytes, _ := getDirSize(pkgPath)

				apps = append(apps, AppInfo{
					Manager:    "brew",
					Name:       pkg.Name(),
					Version:    latestVer,
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

	return apps, nil
}
