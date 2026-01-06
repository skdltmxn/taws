package common

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func DefaultDownloadBaseDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		if tmp := os.TempDir(); tmp != "" {
			return filepath.Join(tmp, "taws-downloads"), nil
		}
		return "", err
	}

	downloads := filepath.Join(home, "Downloads")
	if st, err := os.Stat(downloads); err == nil && st.IsDir() {
		return filepath.Join(downloads, "taws-downloads"), nil
	}
	return filepath.Join(home, "taws-downloads"), nil
}

func S3DownloadPath(bucket, key string) (string, error) {
	if bucket == "" || key == "" {
		return "", fmt.Errorf("invalid bucket or key")
	}

	base, err := DefaultDownloadBaseDir()
	if err != nil {
		return "", err
	}

	dir := path.Dir(key)
	name := path.Base(key)
	if name == "." || name == "/" || name == "" {
		return "", fmt.Errorf("invalid key")
	}

	parts := []string{base, sanitizePathPart(bucket)}
	if dir != "." && dir != "/" && dir != "" {
		for _, p := range strings.Split(dir, "/") {
			p = strings.TrimSpace(p)
			if p == "" || p == "." {
				continue
			}
			parts = append(parts, sanitizePathPart(p))
		}
	}
	parts = append(parts, sanitizePathPart(name))
	dst := filepath.Join(parts...)
	return UniquePath(dst), nil
}

func sanitizePathPart(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "_"
	}
	s = strings.TrimRight(s, ". ")
	if s == "" {
		s = "_"
	}
	b := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			b = append(b, r)
		case r >= 'A' && r <= 'Z':
			b = append(b, r)
		case r >= '0' && r <= '9':
			b = append(b, r)
		case r == '-' || r == '_' || r == '.' || r == '(' || r == ')' || r == ' ':
			b = append(b, r)
		default:
			b = append(b, '_')
		}
	}
	out := strings.TrimSpace(string(b))
	if out == "" {
		return "_"
	}
	return out
}

func UniquePath(p string) string {
	if _, err := os.Stat(p); err != nil {
		return p
	}

	dir := filepath.Dir(p)
	base := filepath.Base(p)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	for i := 1; i < 10000; i++ {
		cand := filepath.Join(dir, fmt.Sprintf("%s-%d%s", name, i, ext))
		if _, err := os.Stat(cand); err != nil {
			return cand
		}
	}
	return p
}

func ExpandUserPath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", nil
	}
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, "~\\") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if p == "~" {
			return home, nil
		}
		return filepath.Join(home, p[2:]), nil
	}
	return p, nil
}
