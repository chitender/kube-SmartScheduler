package version

import (
	"fmt"
	"runtime"
)

var (
	// These variables are set via ldflags during build
	Version    = "unknown"
	CommitHash = "unknown"
	BuildDate  = "unknown"
)

// Info holds version information
type Info struct {
	Version    string `json:"version"`
	CommitHash string `json:"commitHash"`
	BuildDate  string `json:"buildDate"`
	GoVersion  string `json:"goVersion"`
	Compiler   string `json:"compiler"`
	Platform   string `json:"platform"`
}

// Get returns version information
func Get() Info {
	return Info{
		Version:    Version,
		CommitHash: CommitHash,
		BuildDate:  BuildDate,
		GoVersion:  runtime.Version(),
		Compiler:   runtime.Compiler,
		Platform:   fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}

// String returns a formatted version string
func (i Info) String() string {
	return fmt.Sprintf("Version: %s, Commit: %s, Built: %s, Go: %s, Platform: %s",
		i.Version, i.CommitHash, i.BuildDate, i.GoVersion, i.Platform)
}

// GetVersionString returns just the version string
func GetVersionString() string {
	return Version
}
