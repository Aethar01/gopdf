package mupdf

import (
	"strings"
	"testing"

	"gopdf/internal/testpdf"
)

func TestClosedDocumentReturnsErrors(t *testing.T) {
	doc := &Document{}

	if _, err := doc.PageCount(); err == nil {
		t.Fatal("expected PageCount on closed document to fail")
	}
	if _, err := doc.PageInfo(0); err == nil {
		t.Fatal("expected PageInfo on closed document to fail")
	}
	if _, err := doc.Metadata(); err == nil {
		t.Fatal("expected Metadata on closed document to fail")
	}
	if _, err := doc.NewRenderer(); err == nil {
		t.Fatal("expected NewRenderer on closed document to fail")
	}
	if _, err := doc.ExtractSelection(0, Point{}, Point{}, SelectChars); err == nil {
		t.Fatal("expected ExtractSelection on closed document to fail")
	}
	if _, err := doc.SearchPage(0, "needle"); err == nil {
		t.Fatal("expected SearchPage on closed document to fail")
	}
	if _, err := doc.PageText(0); err == nil {
		t.Fatal("expected PageText on closed document to fail")
	}
	if _, err := doc.Links(0); err == nil {
		t.Fatal("expected Links on closed document to fail")
	}
	if _, err := doc.Outline(); err == nil {
		t.Fatal("expected Outline on closed document to fail")
	}
}

func TestSupportedExtensionsComeFromTheBuild(t *testing.T) {
	extensions := SupportedExtensions()
	if len(extensions) == 0 {
		t.Fatal("no document handlers were recognised")
	}
	// PDF is the one format any usable MuPDF build must have.
	if !SupportsExtension("pdf") {
		t.Fatalf("pdf is not supported; extensions = %v", extensions)
	}
	for _, form := range []string{"pdf", ".pdf", "PDF", ".PDF"} {
		if !SupportsExtension(form) {
			t.Errorf("SupportsExtension(%q) = false, want true", form)
		}
	}
	if SupportsExtension("not-a-format") {
		t.Error("an unknown extension was reported as supported")
	}
	if !SupportsPath("/books/paper.PDF") {
		t.Error("SupportsPath should match case-insensitively")
	}
	if SupportsPath("/books/no-extension") {
		t.Error("a file with no extension should not match by name")
	}
	// The list is sorted and every entry is bare and lower-case.
	for i, ext := range extensions {
		if strings.HasPrefix(ext, ".") || ext != strings.ToLower(ext) {
			t.Errorf("extension %q is not bare lower-case", ext)
		}
		if i > 0 && extensions[i-1] >= ext {
			t.Errorf("extensions are not sorted: %q before %q", extensions[i-1], ext)
		}
	}
	// The candidate list is a superset filtered by the build, never the reverse.
	candidates := map[string]bool{}
	for _, ext := range candidateExtensions {
		candidates[ext] = true
	}
	for _, ext := range extensions {
		if !candidates[ext] {
			t.Errorf("reported %q which is not a candidate", ext)
		}
	}
}

func TestPageInfoAppliesCropBoxRotationAndLabels(t *testing.T) {
	path := testpdf.WriteObjects(t, []string{
		"<</Type/Catalog/Pages 2 0 R/PageLabels<</Nums[0<</S/r>>1<</P(A-)/S/D>>]>>>>",
		"<</Type/Pages/Kids[3 0 R 4 0 R]/Count 2/Rotate 90>>", // pages inherit the rotation
		"<</Type/Page/Parent 2 0 R/MediaBox[0 0 612 792]/CropBox[10 20 310 420]>>",
		"<</Type/Page/Parent 2 0 R/MediaBox[0 0 612 792]/Rotate 0/UserUnit 2>>",
	})
	doc, err := Open(path, OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer doc.Close()
	for page, want := range []PageInfo{
		{Bounds: Rect{X1: 400, Y1: 300}, Label: "i"},
		{Bounds: Rect{X1: 1224, Y1: 1584}, Label: "A-1"},
	} {
		info, err := doc.PageInfo(page)
		if err != nil || info != want {
			t.Errorf("page %d: PageInfo = %+v, %v; want %+v", page, info, err, want)
		}
	}
}
