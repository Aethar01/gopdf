//go:build !darwin

package viewer

// noteSystemRecentDocument adds path to the system's recent documents, on
// systems gopdf keeps them for.
func noteSystemRecentDocument(path string) {}
