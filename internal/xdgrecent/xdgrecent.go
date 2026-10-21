// Package xdgrecent adds files to the recent files freedesktop.org desktops
// share, the XBEL file GTK's file chooser, file managers and other
// applications read and write.
//
// The file is shared, so it is edited in place rather than decoded and
// written out again, which would drop what other applications keep in it:
// an entry already there for the file gains onon as an application, and
// otherwise one is added before the end.
package xdgrecent

import (
	"fmt"
	"html"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// App is the name and command onon is recorded under.
const (
	App     = "onon"
	AppExec = "'onon %u'"
)

const emptyFile = `<?xml version="1.0" encoding="UTF-8"?>
<xbel version="1.0"
      xmlns:bookmark="http://www.freedesktop.org/standards/desktop-bookmarks"
      xmlns:mime="http://www.freedesktop.org/standards/shared-mime-info"
>
</xbel>
`

var mu sync.Mutex

// Path returns the shared recent files list.
func Path() (string, error) {
	if dir := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(dir) {
		return filepath.Join(dir, "recently-used.xbel"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "recently-used.xbel"), nil
}

// Add records file as used by onon now.
func Add(file string) error {
	list, err := Path()
	if err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	data, err := os.ReadFile(list)
	if os.IsNotExist(err) {
		data, err = []byte(emptyFile), nil
	}
	if err != nil {
		return err
	}
	updated, err := add(data, file, time.Now())
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(list), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(list), ".recently-used.xbel.*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(updated); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), list)
}

var (
	bookmarkEnd    = "</bookmark>"
	applicationsAt = regexp.MustCompile(`</bookmark:applications>`)
	timeAttr       = regexp.MustCompile(`\b(modified|visited)="[^"]*"`)
	countAttr      = regexp.MustCompile(`\bcount="(\d+)"`)
)

// add returns data, an XBEL file, with file recorded as used at now.
func add(data []byte, file string, now time.Time) ([]byte, error) {
	text := string(data)
	uri := (&url.URL{Scheme: "file", Path: file}).String()
	stamp := now.UTC().Format("2006-01-02T15:04:05.000000Z")
	attr := html.EscapeString(uri)

	open := strings.Index(text, `<bookmark href="`+attr+`"`)
	if open < 0 {
		end := strings.LastIndex(text, "</xbel>")
		if end < 0 {
			return nil, fmt.Errorf("not an XBEL file")
		}
		return []byte(text[:end] + newBookmark(attr, file, stamp) + text[end:]), nil
	}
	closeAt := strings.Index(text[open:], bookmarkEnd)
	if closeAt < 0 {
		return nil, fmt.Errorf("unterminated bookmark for %s", uri)
	}
	block := text[open : open+closeAt]
	tagEnd := strings.IndexByte(block, '>')
	if tagEnd < 0 {
		return nil, fmt.Errorf("unterminated bookmark for %s", uri)
	}
	tag := timeAttr.ReplaceAllString(block[:tagEnd], `$1="`+stamp+`"`)
	body := block[tagEnd:]
	if app := strings.Index(body, `<bookmark:application name="`+App+`"`); app >= 0 {
		appEnd := app + strings.Index(body[app:], ">")
		appTag := timeAttr.ReplaceAllString(body[app:appEnd], `$1="`+stamp+`"`)
		appTag = countAttr.ReplaceAllStringFunc(appTag, func(count string) string {
			n, _ := strconv.Atoi(countAttr.FindStringSubmatch(count)[1])
			return `count="` + strconv.Itoa(n+1) + `"`
		})
		body = body[:app] + appTag + body[appEnd:]
	} else if loc := applicationsAt.FindStringIndex(body); loc != nil {
		body = body[:loc[0]] + "  " + application(stamp) + "\n        " + body[loc[0]:]
	}
	return []byte(text[:open] + tag + body + text[open+closeAt:]), nil
}

func application(stamp string) string {
	return fmt.Sprintf(`<bookmark:application name="%s" exec="%s" modified="%s" count="1"/>`,
		App, html.EscapeString(AppExec), stamp)
}

func newBookmark(href, file, stamp string) string {
	mimeType := mime.TypeByExtension(filepath.Ext(file))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	mimeType, _, _ = strings.Cut(mimeType, ";")
	return fmt.Sprintf(`  <bookmark href="%s" added="%s" modified="%s" visited="%s">
    <info>
      <metadata owner="http://freedesktop.org">
        <mime:mime-type type="%s"/>
        <bookmark:applications>
          %s
        </bookmark:applications>
      </metadata>
    </info>
  </bookmark>
`, href, stamp, stamp, stamp, html.EscapeString(mimeType), application(stamp))
}
