package viewer

import (
	"fmt"
	"math"

	"gopdf/internal/config"
	"gopdf/internal/mupdf"
)

func (a *App) Metadata() (config.DocumentMetadata, error) {
	a.documentAPIMu.Lock()
	defer a.documentAPIMu.Unlock()
	if a.doc == nil {
		return config.DocumentMetadata{}, fmt.Errorf("document unavailable")
	}
	metadata, err := a.doc.Metadata()
	if err != nil {
		return config.DocumentMetadata{}, err
	}
	return config.DocumentMetadata{
		Format: metadata.Format, Encryption: metadata.Encryption, Title: metadata.Title, Author: metadata.Author,
		Subject: metadata.Subject, Keywords: metadata.Keywords, Creator: metadata.Creator, Producer: metadata.Producer,
		CreationDate: metadata.CreationDate, ModificationDate: metadata.ModificationDate,
	}, nil
}

func (a *App) Outline() ([]config.DocumentOutlineItem, error) {
	a.documentAPIMu.Lock()
	defer a.documentAPIMu.Unlock()
	if a.doc == nil {
		return nil, fmt.Errorf("document unavailable")
	}
	if a.outline == nil {
		outline, err := a.doc.Outline()
		if err != nil {
			return nil, err
		}
		a.outline = outline
	}
	items := make([]config.DocumentOutlineItem, len(a.outline))
	children := make([][]int, len(a.outline))
	rootIndices := make([]int, 0)
	for i, item := range a.outline {
		items[i] = config.DocumentOutlineItem{Title: item.Title, URI: item.URI, External: item.External, Page: documentPage(item.Page), X: optionalCoordinate(item.X, item.HasX), Y: optionalCoordinate(item.Y, item.HasY)}
		parent := item.Parent
		if parent >= 0 && parent < i {
			children[parent] = append(children[parent], i)
		} else {
			rootIndices = append(rootIndices, i)
		}
	}
	var build func(int) config.DocumentOutlineItem
	build = func(index int) config.DocumentOutlineItem {
		item := items[index]
		for _, child := range children[index] {
			item.Children = append(item.Children, build(child))
		}
		return item
	}
	roots := make([]config.DocumentOutlineItem, 0, len(rootIndices))
	for _, index := range rootIndices {
		roots = append(roots, build(index))
	}
	return roots, nil
}

func (a *App) PageInfo(page int) (config.DocumentPageInfo, error) {
	a.documentAPIMu.Lock()
	defer a.documentAPIMu.Unlock()
	index, err := a.documentPageIndex(page)
	if err != nil {
		return config.DocumentPageInfo{}, err
	}
	metric := a.pageMetrics[index]
	if !metric.loaded {
		bounds, boundsErr := a.doc.Bounds(index)
		if boundsErr != nil {
			return config.DocumentPageInfo{}, boundsErr
		}
		label, labelErr := a.doc.PageLabel(index)
		if labelErr != nil {
			return config.DocumentPageInfo{}, labelErr
		}
		metric.bounds, metric.label, metric.loaded = bounds, label, true
		metric.width, metric.height = rotatedBoundsSize(bounds, 0)
		a.pageMetrics[index] = metric
	}
	return config.DocumentPageInfo{Page: page, Label: metric.label, Width: float64(metric.bounds.X1 - metric.bounds.X0), Height: float64(metric.bounds.Y1 - metric.bounds.Y0), Bounds: documentRect(metric.bounds)}, nil
}

// Selection reports the current text selection. It reads viewer state owned by
// the main thread, so it is exposed synchronously rather than through a
// plugin operation.
func (a *App) Selection() (config.DocumentSelection, error) {
	selection := config.DocumentSelection{Active: a.selection.active, Text: a.selection.text}
	if a.selection.active || a.selection.text != "" || len(a.selection.quads) > 0 {
		selection.Page = a.selection.page + 1
	}
	selection.Quads = make([]config.DocumentRect, len(a.selection.quads))
	for i, quad := range a.selection.quads {
		selection.Quads[i] = quadRect(quad)
	}
	return selection, nil
}

func (a *App) PageText(page int) (string, error) {
	a.documentAPIMu.Lock()
	defer a.documentAPIMu.Unlock()
	index, err := a.documentPageIndex(page)
	if err != nil {
		return "", err
	}
	return a.doc.PageText(index)
}

func (a *App) PageLinks(page int) ([]config.DocumentPageLink, error) {
	a.documentAPIMu.Lock()
	defer a.documentAPIMu.Unlock()
	index, err := a.documentPageIndex(page)
	if err != nil {
		return nil, err
	}
	links, err := a.linksForPageLocked(index)
	if err != nil {
		return nil, err
	}
	result := make([]config.DocumentPageLink, len(links))
	for i, link := range links {
		result[i] = config.DocumentPageLink{Bounds: documentRect(link.Bounds), URI: link.URI, External: link.External, Page: documentPage(link.Page), X: optionalCoordinate(link.X, link.HasX), Y: optionalCoordinate(link.Y, link.HasY)}
	}
	return result, nil
}

func (a *App) documentPageIndex(page int) (int, error) {
	if a.doc == nil {
		return 0, fmt.Errorf("document unavailable")
	}
	if page < 1 || page > a.pageCount {
		return 0, fmt.Errorf("page %d out of range [1,%d]", page, a.pageCount)
	}
	return page - 1, nil
}

func documentPage(page int) int {
	if page < 0 {
		return 0
	}
	return page + 1
}

func optionalCoordinate(value float64, present bool) *float64 {
	if !present {
		return nil
	}
	return &value
}

func quadRect(quad mupdf.Quad) config.DocumentRect {
	x0 := math.Min(math.Min(float64(quad.UL.X), float64(quad.UR.X)), math.Min(float64(quad.LL.X), float64(quad.LR.X)))
	y0 := math.Min(math.Min(float64(quad.UL.Y), float64(quad.UR.Y)), math.Min(float64(quad.LL.Y), float64(quad.LR.Y)))
	x1 := math.Max(math.Max(float64(quad.UL.X), float64(quad.UR.X)), math.Max(float64(quad.LL.X), float64(quad.LR.X)))
	y1 := math.Max(math.Max(float64(quad.UL.Y), float64(quad.UR.Y)), math.Max(float64(quad.LL.Y), float64(quad.LR.Y)))
	return config.DocumentRect{X0: x0, Y0: y0, X1: x1, Y1: y1}
}

func documentRect(rect mupdf.Rect) config.DocumentRect {
	return config.DocumentRect{X0: float64(rect.X0), Y0: float64(rect.Y0), X1: float64(rect.X1), Y1: float64(rect.Y1)}
}

// SupportedExtensions and SupportsPath report what the linked MuPDF build can
// open, so pickers and plugins offer the real format set rather than assuming
// PDF only.
func (a *App) SupportedExtensions() []string { return mupdf.SupportedExtensions() }

func (a *App) SupportsPath(path string) bool { return mupdf.SupportsPath(path) }
