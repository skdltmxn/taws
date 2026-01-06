package buildinfo

import "fmt"

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func UIString() string {
	v := Version
	if v == "" {
		v = "dev"
	}
	if v == "dev" && Commit != "" && Commit != "none" {
		return fmt.Sprintf("%s (%s)", v, short(Commit))
	}
	return v
}

func FullString() string {
	v := Version
	if v == "" {
		v = "dev"
	}
	c := Commit
	if c == "" {
		c = "none"
	}
	d := Date
	if d == "" {
		d = "unknown"
	}
	return fmt.Sprintf("%s (commit %s, built %s)", v, c, d)
}

func short(s string) string {
	if len(s) <= 12 {
		return s
	}
	return s[:12]
}
