package filepicker

import "github.com/sqweek/dialog"

func PickPDF() (string, error) {
	return dialog.File().Title("Open PDF").Load()
}
