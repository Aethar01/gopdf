package filepicker

import "github.com/sqweek/dialog"

// PickDocument opens the native file picker. The extensions, without a leading
// dot, become a filter for the formats the caller can open; passing none offers
// every file. The caller supplies them so this package stays free of MuPDF.
func PickDocument(extensions []string) (string, error) {
	builder := dialog.File().Title("Open Document")
	if len(extensions) > 0 {
		builder = builder.Filter("Documents", extensions...)
	}
	return builder.Load()
}

func PickDirectory() (string, error) {
	return dialog.Directory().Title("Select Directory").Browse()
}
