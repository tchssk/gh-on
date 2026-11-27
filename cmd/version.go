package cmd

// Version is the current version of gh-on.
// In release builds, this is overridden at build time via ldflags
// (see script/build.sh).
// The "dev" default indicates a local development build.
var Version = "dev"
