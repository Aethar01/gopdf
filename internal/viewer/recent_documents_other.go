//go:build !darwin && !linux && !windows

package viewer

// noteSystemRecentDocument adds path to the system's recent documents, on
// systems onon keeps them for.
func noteSystemRecentDocument(path string) {}
