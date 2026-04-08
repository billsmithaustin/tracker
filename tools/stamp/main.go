// Rewrites ?v=... cache-busters on local asset URLs in HTML files
// using each asset's modification timestamp. Run before deploying.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func main() {
	frontend := "./frontend"
	if env := os.Getenv("FRONTEND"); env != "" {
		frontend = env
	}

	pages := []string{"index.html", "checkin.html"}
	// Matches href/src to a local (non-http) asset with optional existing ?v=...
	assetRe := regexp.MustCompile(`(href|src)="(/[^"?]+\.(css|js))(?:\?v=[^"]*)?">`)

	for _, page := range pages {
		file := filepath.Join(frontend, page)
		data, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading %s: %v\n", file, err)
			os.Exit(1)
		}
		html := string(data)

		// Replace each asset reference using submatch indices for precision.
		var result strings.Builder
		last := 0
		for _, idx := range assetRe.FindAllStringSubmatchIndex(html, -1) {
			// idx[0:2] = full match, [2:4] = attr, [4:6] = assetPath, [6:8] = ext
			result.WriteString(html[last:idx[0]])
			attr := html[idx[2]:idx[3]]
			assetPath := html[idx[4]:idx[5]]

			localPath := filepath.Join(frontend, assetPath)
			info, err := os.Stat(localPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  skipping %s (not found locally)\n", assetPath)
				result.WriteString(html[idx[0]:idx[1]])
			} else {
				mtime := info.ModTime().Unix()
				fmt.Printf("  %s?v=%d\n", assetPath, mtime)
				fmt.Fprintf(&result, `%s="%s?v=%d">`, attr, assetPath, mtime)
			}
			last = idx[1]
		}
		result.WriteString(html[last:])

		if err := os.WriteFile(file, []byte(result.String()), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "error writing %s: %v\n", file, err)
			os.Exit(1)
		}
		fmt.Printf("Stamped %s\n", page)
	}
}
