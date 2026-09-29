package viewer

import (
	"testing"
	"time"

	"gopdf/internal/mupdf"
)

func TestLinkPreviewAppearsAfterDelayAndRequestsTiles(t *testing.T) {
	app := testPrefetchApp(5, 1)
	app.config.LinkPreview, app.config.LinkPreviewDelayMS = true, 400
	link := mupdf.Link{Page: 3, Y: 50, HasY: true}

	app.hoverLinkPreview(link, true, 100, 100)
	if app.previewShown() || app.previewDeadline().IsZero() {
		t.Fatal("preview shown before the delay")
	}
	app.preview.since = time.Now().Add(-app.linkPreviewDelay())
	app.revealLinkPreview()
	if !app.previewShown() || !app.pendingRedraw {
		t.Fatal("preview not revealed after the delay")
	}
	app.prefetchVisiblePages()
	requested := false
	for key, req := range app.renderPending {
		requested = requested || key.page == 3 && req.priority == 0
	}
	if !requested {
		t.Fatalf("no on-screen tiles requested for the previewed page: %v", app.renderPending)
	}

	popup, _, y := app.previewPlacement()
	if y+50*app.scale != float64(popup.Y)+16 {
		t.Fatalf("destination drawn at y=%.1f, want popup top + 16 (%.1f)", y+50*app.scale, popup.Y+16)
	}

	app.hoverLinkPreview(mupdf.Link{External: true, URI: "https://x"}, true, 0, 0)
	if app.preview != nil {
		t.Fatal("external links are not previewed")
	}
}
