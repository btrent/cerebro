// Package images handles image attachments: detecting image file paths in user
// input and loading them into llm.Image values for multimodal requests.
package images

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cerebro/internal/llm"
)

// supported maps recognized extensions to MIME types.
var supported = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".webp": "image/webp",
	".gif":  "image/gif",
}

// IsImagePath reports whether s ends in a supported image extension.
func IsImagePath(s string) bool {
	_, ok := supported[strings.ToLower(filepath.Ext(s))]
	return ok
}

// MIMEFor returns the MIME type for a path's extension, or "" if unsupported.
func MIMEFor(path string) string {
	return supported[strings.ToLower(filepath.Ext(path))]
}

// DetectPaths scans whitespace-separated tokens in input and returns those that
// look like image file paths. Surrounding quotes are stripped. Order is
// preserved and duplicates are removed.
func DetectPaths(input string) []string {
	seen := map[string]bool{}
	var out []string
	for _, tok := range strings.Fields(input) {
		tok = strings.Trim(tok, `"'`)
		if IsImagePath(tok) && !seen[tok] {
			seen[tok] = true
			out = append(out, tok)
		}
	}
	return out
}

// Load reads an image file into an llm.Image, expanding a leading ~ and
// returning a clear error if the file is missing or unsupported.
func Load(path string) (llm.Image, error) {
	expanded := expandHome(path)
	mime := MIMEFor(expanded)
	if mime == "" {
		return llm.Image{}, fmt.Errorf("unsupported image type: %s", path)
	}
	data, err := os.ReadFile(expanded)
	if err != nil {
		if os.IsNotExist(err) {
			return llm.Image{}, fmt.Errorf("image not found: %s", path)
		}
		return llm.Image{}, fmt.Errorf("reading image %s: %w", path, err)
	}
	return llm.Image{Path: path, MIME: mime, Data: data}, nil
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(p, "~/"))
		}
	}
	return p
}
