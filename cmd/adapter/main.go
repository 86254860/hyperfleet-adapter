package adapter

// Build-time variables set via ldflags (reserved for future version/build info display)
var (
	_ = version   // Unused: reserved for --version flag
	_ = commit    // Unused: reserved for build info
	_ = buildDate // Unused: reserved for build info
	_ = tag       // Unused: reserved for build info

	version   string
	commit    string
	buildDate string
	tag       string
)
