package viewer

import (
	"slices"
	"strings"
	"unicode/utf8"

	"gopdf/internal/config"
	"gopdf/internal/mupdf"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

// Link hints label every link on screen; typing a label follows it.

type linkHint struct {
	label string
	page  int
	link  mupdf.Link
}

type hintState struct {
	hints []linkHint
	typed string
}

func (a *App) startLinkHints() {
	var hints []linkHint
	viewportW, viewportH := a.viewportSize()
	a.forEachDisplayedPage(0, func(page int, x, y float64) {
		links, err := a.linksForPage(page)
		if err != nil {
			a.logf("load links page=%d err=%v", page+1, err)
			return
		}
		for _, link := range links {
			minX, minY, maxX, maxY := a.quadScreenBounds(rectQuad(link.Bounds), page, x, y)
			if maxX >= 0 && maxY >= 0 && minX <= float64(viewportW) && minY <= float64(viewportH) {
				hints = append(hints, linkHint{page: page, link: link})
			}
		}
	})
	if len(hints) == 0 {
		a.message = "no links on screen"
		return
	}
	for i, label := range hintLabels(len(hints), a.config.HintChars) {
		hints[i].label = label
	}
	a.hints = &hintState{hints: hints}
	a.message = "follow link (Esc cancels)"
}

// hintLabels returns n distinct labels of equal length drawn from chars,
// as short as chars allows.
func hintLabels(n int, chars string) []string {
	var alphabet []rune
	for _, r := range chars {
		if !slices.Contains(alphabet, r) {
			alphabet = append(alphabet, r)
		}
	}
	if len(alphabet) < 2 {
		alphabet = []rune(config.Default().HintChars)
	}
	length := 1
	for capacity := len(alphabet); capacity < n; capacity *= len(alphabet) {
		length++
	}
	labels := make([]string, n)
	for i := range labels {
		label := make([]rune, length)
		for j, v := length-1, i; j >= 0; j, v = j-1, v/len(alphabet) {
			label[j] = alphabet[v%len(alphabet)]
		}
		labels[i] = string(label)
	}
	return labels
}

// handleHintToken consumes every key while hints are shown: hint characters
// narrow the hints, Backspace undoes one, and Esc cancels.
func (a *App) handleHintToken(token string) bool {
	if a.hints == nil {
		return false
	}
	switch normalizeBinding(token) {
	case normalizeBinding("<Esc>"):
		a.cancelLinkHints()
		return true
	case normalizeBinding("<BS>"):
		if _, size := utf8.DecodeLastRuneInString(a.hints.typed); size > 0 {
			a.hints.typed = a.hints.typed[:len(a.hints.typed)-size]
		}
		return true
	}
	typed := a.hints.typed + token
	matched := false
	for _, hint := range a.hints.hints {
		if hint.label == typed {
			a.cancelLinkHints()
			a.activateLink(hint.link)
			return true
		}
		matched = matched || strings.HasPrefix(hint.label, typed)
	}
	if matched {
		a.hints.typed = typed
	}
	return true
}

func (a *App) cancelLinkHints() {
	a.hints = nil
	a.message = ""
}

func (a *App) drawLinkHints(renderer *sdl.Renderer) {
	if a.hints == nil {
		return
	}
	metrics := a.fontFace.Metrics()
	ascent, descent := metrics.Ascent.Ceil(), metrics.Descent.Ceil()
	bg, fg := a.selectionColor(), a.highlightForegroundColor()
	bg.A = 0xff
	typedFG := fg
	typedFG.A /= 2
	for _, hint := range a.hints.hints {
		if !strings.HasPrefix(hint.label, a.hints.typed) {
			continue
		}
		x, y, ok := a.pageScreenOrigin(hint.page)
		if !ok {
			continue
		}
		minX, minY, _, _ := a.quadScreenBounds(rectQuad(hint.link.Bounds), hint.page, x, y)
		const pad = 3
		w := measureText(a.fontFace, hint.label) + 2*pad
		box := sdl.FRect{X: float32(minX), Y: float32(minY), W: float32(w), H: float32(ascent + descent + 2*pad)}
		fillRect(renderer, box, bg)
		strokeRect(renderer, box, fg, 1)
		baseline := int(minY) + pad + ascent
		typedW := measureText(a.fontFace, a.hints.typed)
		a.drawText(renderer, a.hints.typed, int(minX)+pad, baseline, typedFG)
		a.drawText(renderer, hint.label[len(a.hints.typed):], int(minX)+pad+typedW, baseline, fg)
	}
}

func rectQuad(r mupdf.Rect) mupdf.Quad {
	x0, y0, x1, y1 := float64(r.X0), float64(r.Y0), float64(r.X1), float64(r.Y1)
	return mupdf.Quad{UL: mupdf.Point{X: x0, Y: y0}, UR: mupdf.Point{X: x1, Y: y0}, LL: mupdf.Point{X: x0, Y: y1}, LR: mupdf.Point{X: x1, Y: y1}}
}
