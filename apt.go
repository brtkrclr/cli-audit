package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// APT / dpkg parsing logic (Linux support)
func getAptApps(stats map[string]CommandStats) ([]AppInfo, error) {
	if _, err := exec.LookPath("dpkg-query"); err != nil {
		return nil, fmt.Errorf("dpkg-query not found")
	}

	cmd := exec.Command("dpkg-query", "-W", "-f=${Package}|${Version}|${Status}|${Installed-Size}\n")
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
		if len(parts) >= 3 && strings.Contains(parts[2], "installed") {
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

			var sizeBytes int64
			if len(parts) >= 4 {
				sizeKb, _ := strconv.ParseInt(parts[3], 10, 64)
				sizeBytes = sizeKb * 1024
			}

			apps = append(apps, AppInfo{
				Manager:    "apt",
				Name:       nameBase,
				Version:    version,
				Time:       installTime,
				Binaries:   bins,
				UsageCount: totalUsage,
				LastUsed:   lastUsed,
				SizeBytes:  sizeBytes,
				SizeString: formatSize(sizeBytes),
			})
		}
	}

	return apps, nil
}
