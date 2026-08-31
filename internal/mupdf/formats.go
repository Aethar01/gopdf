package mupdf

/*
#include <stdlib.h>
#include "mupdf_bridge.h"
*/
import "C"

import (
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unsafe"
)

// candidateExtensions are the names MuPDF may have handlers for. The build
// decides which are real: each is offered to fz_recognize_document and only the
// accepted ones are reported, so a MuPDF compiled without EPUB or image support
// does not advertise formats it cannot open.
var candidateExtensions = []string{
	"pdf",
	"xps", "oxps",
	"epub", "mobi", "prc", "azw", "azw3", "fb2",
	"cbz", "cbr", "cbt", "zip", "tar",
	"html", "htm", "xhtml", "xml",
	"svg", "txt",
	"png", "jpg", "jpeg", "jfif", "jpe",
	"gif", "bmp", "tif", "tiff",
	"pnm", "pgm", "ppm", "pbm", "pam",
	"jpx", "jp2", "jxr", "wdp", "hdp",
	"psd", "webp", "heic", "heif", "avif",
	"pkm", "ktx", "ktx2",
	"docx", "xlsx", "pptx",
}

var (
	formatsOnce  sync.Once
	formatProbe  sync.Mutex
	supportedSet map[string]bool
	supportedExt []string
)

func loadSupportedFormats() {
	formatsOnce.Do(func() {
		supportedSet = make(map[string]bool, len(candidateExtensions))
		for _, ext := range candidateExtensions {
			if recognizeName("document." + ext) {
				supportedSet[ext] = true
			}
		}
		supportedExt = make([]string, 0, len(supportedSet))
		for ext := range supportedSet {
			supportedExt = append(supportedExt, ext)
		}
		sort.Strings(supportedExt)
	})
}

// recognizeName asks MuPDF whether any registered handler claims this name.
// The bridge keeps one probe context, so calls are serialised here.
func recognizeName(name string) bool {
	formatProbe.Lock()
	defer formatProbe.Unlock()
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	return C.gopdf_recognize_document_name(cname) != 0
}

// SupportedExtensions returns the lower-case extensions, without a leading dot,
// that this MuPDF build has document handlers for. The result is sorted and
// must not be modified.
func SupportedExtensions() []string {
	loadSupportedFormats()
	return append([]string(nil), supportedExt...)
}

// SupportsExtension reports whether an extension has a document handler. It
// accepts ".pdf", "pdf", or "PDF".
func SupportsExtension(ext string) bool {
	loadSupportedFormats()
	return supportedSet[normalizeExtension(ext)]
}

// SupportsPath reports whether a path's extension has a document handler. This
// is a name-based hint for listings and pickers; opening still sniffs content,
// so a file with no or a misleading extension may still open.
func SupportsPath(path string) bool {
	return SupportsExtension(filepath.Ext(path))
}

func normalizeExtension(ext string) string {
	return strings.ToLower(strings.TrimPrefix(strings.TrimSpace(ext), "."))
}
