// Package testpdf writes minimal single-page PDFs for tests.
package testpdf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Write creates a one-page Letter PDF with each line set in 12pt Helvetica,
// starting at (72, 700) with 14pt leading, and returns its path.
func Write(t testing.TB, lines ...string) string {
	t.Helper()
	var content strings.Builder
	content.WriteString("BT /F1 12 Tf 14 TL 72 700 Td")
	for _, line := range lines {
		r := strings.NewReplacer(`\`, `\\`, `(`, `\(`, `)`, `\)`)
		fmt.Fprintf(&content, " (%s) Tj T*", r.Replace(line))
	}
	content.WriteString(" ET")

	objects := []string{
		"<</Type/Catalog/Pages 2 0 R>>",
		"<</Type/Pages/Kids[3 0 R]/Count 1>>",
		"<</Type/Page/Parent 2 0 R/MediaBox[0 0 612 792]/Resources<</Font<</F1 4 0 R>>>>/Contents 5 0 R>>",
		"<</Type/Font/Subtype/Type1/BaseFont/Helvetica>>",
		fmt.Sprintf("<</Length %d>>\nstream\n%s\nendstream", content.Len(), content.String()),
	}
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
