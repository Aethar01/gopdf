// Package testpdf writes minimal PDFs for tests.
package testpdf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Write creates a one-page PDF; see WritePages.
func Write(t testing.TB, lines ...string) string {
	t.Helper()
	return WritePages(t, lines)
}

// WritePages creates a PDF of Letter pages with each line set in 12pt
// Helvetica, starting at (72, 700) with 14pt leading, and returns its path.
func WritePages(t testing.TB, pages ...[]string) string {
	t.Helper()
	escape := strings.NewReplacer(`\`, `\\`, `(`, `\(`, `)`, `\)`)
	// Objects 1-3 are the catalog, page tree and font; each page adds a page
	// object and its content stream.
	kids := make([]string, len(pages))
	objects := []string{"<</Type/Catalog/Pages 2 0 R>>", "", "<</Type/Font/Subtype/Type1/BaseFont/Helvetica>>"}
	for i, lines := range pages {
		var content strings.Builder
		content.WriteString("BT /F1 12 Tf 14 TL 72 700 Td")
		for _, line := range lines {
			fmt.Fprintf(&content, " (%s) Tj T*", escape.Replace(line))
		}
		content.WriteString(" ET")
		pageObj := len(objects) + 1
		kids[i] = fmt.Sprintf("%d 0 R", pageObj)
		objects = append(objects,
			fmt.Sprintf("<</Type/Page/Parent 2 0 R/MediaBox[0 0 612 792]/Resources<</Font<</F1 3 0 R>>>>/Contents %d 0 R>>", pageObj+1),
			fmt.Sprintf("<</Length %d>>\nstream\n%s\nendstream", content.Len(), content.String()),
		)
	}
	objects[1] = fmt.Sprintf("<</Type/Pages/Kids[%s]/Count %d>>", strings.Join(kids, " "), len(pages))

	var pdf strings.Builder
	pdf.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects))
	for i, obj := range objects {
		offsets[i] = pdf.Len()
		fmt.Fprintf(&pdf, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	xref := pdf.Len()
	fmt.Fprintf(&pdf, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, off := range offsets {
		fmt.Fprintf(&pdf, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&pdf, "trailer\n<</Size %d/Root 1 0 R>>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)

	path := filepath.Join(t.TempDir(), "test.pdf")
	if err := os.WriteFile(path, []byte(pdf.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
