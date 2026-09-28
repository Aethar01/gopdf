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

// WriteImage creates a one-page PDF with a red 2x2 image drawn over the
// 100x100pt square whose top-left corner is 100pt from the page's top-left.
func WriteImage(t testing.TB) string {
	t.Helper()
	image := "<</Type/XObject/Subtype/Image/Width 2/Height 2/ColorSpace/DeviceRGB/BitsPerComponent 8/Length 12>>\nstream\n" +
		strings.Repeat("\xff\x00\x00", 4) + "\nendstream"
	content := "q 100 0 0 100 100 592 cm /Im0 Do Q"
	return write(t, []string{
		"<</Type/Catalog/Pages 2 0 R>>",
		"<</Type/Pages/Kids[3 0 R]/Count 1>>",
		"<</Type/Page/Parent 2 0 R/MediaBox[0 0 612 792]/Resources<</XObject<</Im0 4 0 R>>>>/Contents 5 0 R>>",
		image,
		fmt.Sprintf("<</Length %d>>\nstream\n%s\nendstream", len(content), content),
	})
}

// WriteForm creates a one-page PDF with three form fields: a text field
// "name" (value "Ada") over (100,72)-(300,92), a check box "agree" over
// (100,127)-(115,142), and a combo box "colour" (Red or Green) over
// (100,172)-(200,192), in top-left page coordinates.
func WriteForm(t testing.TB) string {
	t.Helper()
	appearance := "<</Type/XObject/Subtype/Form/BBox[0 0 15 15]/Length 0>>\nstream\n\nendstream"
	return write(t, []string{
		"<</Type/Catalog/Pages 2 0 R/AcroForm<</Fields[4 0 R 5 0 R 6 0 R]/DA(/Helv 12 Tf 0 g)/DR<</Font<</Helv 7 0 R>>>>>>>>",
		"<</Type/Pages/Kids[3 0 R]/Count 1>>",
		"<</Type/Page/Parent 2 0 R/MediaBox[0 0 612 792]/Annots[4 0 R 5 0 R 6 0 R]/Contents 8 0 R>>",
		"<</Type/Annot/Subtype/Widget/FT/Tx/T(name)/V(Ada)/Rect[100 700 300 720]/P 3 0 R/DA(/Helv 12 Tf 0 g)/F 4>>",
		"<</Type/Annot/Subtype/Widget/FT/Btn/T(agree)/V/Off/AS/Off/Rect[100 650 115 665]/P 3 0 R/F 4/AP<</N<</Yes 9 0 R/Off 10 0 R>>>>>>",
		"<</Type/Annot/Subtype/Widget/FT/Ch/Ff 131072/T(colour)/Opt[(Red)(Green)]/V(Red)/Rect[100 600 200 620]/P 3 0 R/DA(/Helv 12 Tf 0 g)/F 4>>",
		"<</Type/Font/Subtype/Type1/BaseFont/Helvetica>>",
		"<</Length 0>>\nstream\n\nendstream",
		appearance,
		appearance,
	})
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
	return write(t, objects)
}

// write numbers objects from 1 and writes them as a PDF with an xref table.
func write(t testing.TB, objects []string) string {
	t.Helper()
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
