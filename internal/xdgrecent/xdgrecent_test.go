package xdgrecent

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var at = time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)

const stamp = "2026-10-21T12:00:00.000000Z"

const otherApps = `<?xml version="1.0" encoding="UTF-8"?>
<xbel version="1.0"
      xmlns:bookmark="http://www.freedesktop.org/standards/desktop-bookmarks"
      xmlns:mime="http://www.freedesktop.org/standards/shared-mime-info"
>
  <bookmark href="file:///home/u/a%20b.pdf" added="2026-01-01T00:00:00.000000Z" modified="2026-01-01T00:00:00.000000Z" visited="2026-01-01T00:00:00.000000Z">
    <info>
      <metadata owner="http://freedesktop.org">
        <mime:mime-type type="application/pdf"/>
        <bookmark:applications>
          <bookmark:application name="Document Viewer" exec="&apos;papers %u&apos;" modified="2026-01-01T00:00:00.000000Z" count="3"/>
        </bookmark:applications>
      </metadata>
    </info>
  </bookmark>
  <bookmark href="file:///home/u/notes.txt" added="2026-01-01T00:00:00.000000Z" modified="2026-01-01T00:00:00.000000Z" visited="2026-01-01T00:00:00.000000Z">
    <info>
      <metadata owner="http://freedesktop.org">
        <mime:mime-type type="text/plain"/>
        <bookmark:applications>
          <bookmark:application name="Text Editor" exec="&apos;gnome-text-editor %u&apos;" modified="2026-01-01T00:00:00.000000Z" count="1"/>
        </bookmark:applications>
      </metadata>
    </info>
  </bookmark>
</xbel>
`

func TestAddNewFile(t *testing.T) {
	got, err := add([]byte(emptyFile), "/home/u/book.pdf", at)
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	for _, want := range []string{
		`<bookmark href="file:///home/u/book.pdf" added="` + stamp + `"`,
		`<mime:mime-type type="application/pdf"/>`,
		`<bookmark:application name="gopdf" exec="&#39;gopdf %u&#39;" modified="` + stamp + `" count="1"/>`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %s in\n%s", want, text)
		}
	}
	mustParse(t, got)
}

func TestAddKeepsOtherApplications(t *testing.T) {
	got, err := add([]byte(otherApps), "/home/u/a b.pdf", at)
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	if strings.Count(text, "<bookmark href=") != 2 {
		t.Fatalf("expected the existing entry to be reused:\n%s", text)
	}
	for _, want := range []string{
		`modified="` + stamp + `" visited="` + stamp + `"`,
		`added="2026-01-01T00:00:00.000000Z"`,
		`name="Document Viewer"`,
		`count="3"`,
		`<bookmark:application name="gopdf"`,
		`notes.txt`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %s in\n%s", want, text)
		}
	}
	mustParse(t, got)

	again, err := add(got, "/home/u/a b.pdf", at)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(again), `name="gopdf"`); n != 1 {
		t.Fatalf("gopdf recorded %d times", n)
	}
	if !strings.Contains(string(again), `name="gopdf" exec="&#39;gopdf %u&#39;" modified="`+stamp+`" count="2"`) {
		t.Fatalf("expected gopdf's count to rise:\n%s", again)
	}
}

func TestAddWritesTheSharedFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)
	if err := Add("/home/u/book.pdf"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "recently-used.xbel"))
	if err != nil || !strings.Contains(string(data), "file:///home/u/book.pdf") {
		t.Fatalf("recently-used.xbel holds %q, %v", data, err)
	}
	mustParse(t, data)
}

func mustParse(t *testing.T, data []byte) {
	t.Helper()
	var doc struct {
		Bookmarks []struct {
			Href string `xml:"href,attr"`
		} `xml:"bookmark"`
	}
	if err := xml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("not well-formed XML: %v\n%s", err, data)
	}
}
