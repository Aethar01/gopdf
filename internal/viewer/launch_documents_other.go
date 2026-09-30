//go:build !darwin

package viewer

// LaunchDocuments returns the documents the system launched gopdf to open
// without passing them as arguments. Only macOS does that.
func LaunchDocuments() (paths []string, settled bool) {
	return nil, true
}
