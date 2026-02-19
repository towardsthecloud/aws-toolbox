package version

import "fmt"

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func Short() string {
	return Version
}

func Detailed() string {
	return fmt.Sprintf("version: %s\ncommit: %s\nbuild date: %s", Version, Commit, Date)
}
