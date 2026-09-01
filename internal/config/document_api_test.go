package config

import (
	"path/filepath"
	"strings"
	"testing"
)

type documentAPIHost struct{ stubHost }

func (h *documentAPIHost) Metadata() (DocumentMetadata, error) {
	return DocumentMetadata{Title: "Paper", Author: "Author", Format: "PDF 1.7"}, nil
}

func (h *documentAPIHost) Outline() ([]DocumentOutlineItem, error) {
	return []DocumentOutlineItem{{Title: "Chapter", Page: 1, Children: []DocumentOutlineItem{{Title: "Section", Page: 2}}}}, nil
}

func (h *documentAPIHost) PageInfo(page int) (DocumentPageInfo, error) {
	return DocumentPageInfo{Page: page, Label: "iv", Width: 612, Height: 792, Bounds: DocumentRect{X1: 612, Y1: 792}}, nil
}

func (h *documentAPIHost) Selection() (DocumentSelection, error) {
	return DocumentSelection{Active: true, Page: 3, Text: "selected", Quads: []DocumentRect{{X1: 5, Y1: 6}}}, nil
}

func (h *documentAPIHost) PageText(page int) (string, error) { return "page text", nil }

func (h *documentAPIHost) PageLinks(page int) ([]DocumentPageLink, error) {
	return []DocumentPageLink{{URI: "https://example.com", External: true, Bounds: DocumentRect{X1: 10, Y1: 20}}}, nil
}

func TestLuaDocumentInspection(t *testing.T) {
	rt, err := Open(filepath.Join(t.TempDir(), "missing.lua"), "")
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	rt.AttachHost(&documentAPIHost{})
	if _, err := rt.Eval(`
local metadata = gopdf.document.metadata()
local outline = gopdf.document.outline()
local page = gopdf.document.page_info(4)
assert(metadata.title == "Paper" and metadata.author == "Author")
assert(outline[1].title == "Chapter" and outline[1].children[1].page == 2)
assert(page.page == 4 and page.label == "iv" and page.bounds.x1 == 612)
local selection = gopdf.document.selection()
assert(selection.active and selection.page == 3 and selection.text == "selected")
assert(#selection.quads == 1 and selection.quads[1].y1 == 6)
`); err != nil {
		t.Fatal(err)
	}
}

func TestPluginDocumentExtraction(t *testing.T) {
	root := writeTestPlugin(t, "sample", `return gopdf.plugin.register("sample")`)
	rt, err := OpenWithOptions(filepath.Join(t.TempDir(), "missing.lua"), "", OpenOptions{PluginPaths: []string{root}})
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	rt.AttachHost(&documentAPIHost{})
	if _, err := rt.Eval(`
sample = require("sample")
sample.document:page_text(2, function(result) sample.text_result = result end)
sample.document:page_links(2, function(result) sample.links_result = result end)
`); err != nil {
		t.Fatal(err)
	}
	pollRuntimeUntil(t, rt, func() bool {
		return evalBool(rt, `sample.text_result ~= nil and sample.links_result ~= nil`)
	})
	if _, err := rt.Eval(`
assert(sample.text_result.success and sample.text_result.page == 2 and sample.text_result.text == "page text")
assert(sample.links_result.success and sample.links_result.links[1].external)
assert(sample.links_result.links[1].bounds.y1 == 20)
`); err != nil {
		t.Fatal(err)
	}
}

type formatHost struct{ stubHost }

func (h *formatHost) SupportedExtensions() []string { return []string{"pdf", "epub", "cbz"} }

func (h *formatHost) SupportsPath(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".pdf") ||
		strings.EqualFold(filepath.Ext(path), ".epub") ||
		strings.EqualFold(filepath.Ext(path), ".cbz")
}

func TestLuaFormatsReportsEngineSupport(t *testing.T) {
	rt, err := Open(filepath.Join(t.TempDir(), "missing.lua"), "")
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	rt.AttachHost(&formatHost{})
	if _, err := rt.Eval(`
local extensions = gopdf.formats.extensions()
assert(#extensions == 3 and extensions[1] == "pdf" and extensions[2] == "epub")
assert(gopdf.formats.supports("/books/paper.pdf"))
assert(gopdf.formats.supports("/books/Novel.EPUB"), "matching should ignore case")
assert(gopdf.formats.supports("comic.cbz"))
assert(not gopdf.formats.supports("/books/notes.md"))
assert(not gopdf.formats.supports("/books/no-extension"))
`); err != nil {
		t.Fatal(err)
	}
}

func TestLuaFormatsWithoutADocumentEngine(t *testing.T) {
	rt, err := Open(filepath.Join(t.TempDir(), "missing.lua"), "")
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	rt.AttachHost(&stubHost{})
	if _, err := rt.Eval(`gopdf.formats.extensions()`); err == nil || !strings.Contains(err.Error(), "document engine unavailable") {
		t.Fatalf("err = %v, want one containing \"document engine unavailable\"", err)
	}
}
