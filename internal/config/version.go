package config

// Version is onon's version. Release builds set it to the tag with
// -ldflags "-X onon/internal/config.Version=<tag>"; any other build is dev.
var Version = "dev"
