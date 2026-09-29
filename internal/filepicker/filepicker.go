package filepicker

import (
	"errors"

	"github.com/sqweek/dialog"
)

// PickDocument opens the native file picker and returns "" with a nil error
// when the user cancels. The extensions, without a leading dot, become a filter
// for the formats the caller can open; passing none offers every file. The
// caller supplies them so this package stays free of MuPDF.
func PickDocument(extensions []string) (string, error) {
	builder := dialog.File().Title("Open Document")
	if len(extensions) > 0 {
		builder = builder.Filter("Documents", extensions...)
	}
	return ignoreCancel(builder.Load())
}

// PickDirectory opens the native directory picker and returns "" with a nil
// error when the user cancels.
func PickDirectory() (string, error) {
	return ignoreCancel(dialog.Directory().Title("Select Directory").Browse())
}

func ignoreCancel(path string, err error) (string, error) {
	if errors.Is(err, dialog.ErrCancelled) {
		return "", nil
	}
	return path, err
}
