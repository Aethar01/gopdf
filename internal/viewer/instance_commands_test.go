package viewer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopdf/internal/instance"
	"gopdf/internal/mupdf"
)

func TestApplyInstanceJumpMovesToPage(t *testing.T) {
	app := testLayoutApp(5)
	app.recomputeLayout(app.viewportSize())

	if err := app.applyInstanceJump(instanceJump{page: 4}); err != nil {
		t.Fatalf("jump: %v", err)
	}
	if app.page != 3 {
		t.Fatalf("page = %d, want 3 (1-based 4)", app.page)
	}
	if !app.pendingRedraw {
		t.Error("a jump should request a redraw")
	}
}

func TestApplyInstanceJumpRejectsOutOfRangePages(t *testing.T) {
	app := testLayoutApp(3)
	app.recomputeLayout(app.viewportSize())
	for _, page := range []int{0, -1, 4, 99} {
		if err := app.applyInstanceJump(instanceJump{page: page}); err == nil {
			t.Errorf("page %d was accepted for a 3 page document", page)
		}
	}
}

// A jump that arrives before the document has loaded is held, not rejected,
// because --goto is parsed before any page metrics exist.
func TestPendingJumpIsAppliedOnceThePagesExist(t *testing.T) {
	app := testLayoutApp(0)
	app.QueueStartupJump(3, 0, 0, false)
	app.flushPendingInstanceJump()
	if app.pendingInstanceJump == nil {
		t.Fatal("the jump was dropped while the document had no pages")
	}

	loaded := testLayoutApp(5)
	loaded.pendingInstanceJump = app.pendingInstanceJump
	loaded.recomputeLayout(loaded.viewportSize())
	loaded.flushPendingInstanceJump()
	if loaded.pendingInstanceJump != nil {
		t.Error("the pending jump should be consumed once applied")
	}
	if loaded.page != 2 {
		t.Fatalf("page = %d, want 2 (1-based 3)", loaded.page)
	}
}

func TestQueueStartupJumpIgnoresNonPages(t *testing.T) {
	app := testLayoutApp(5)
	app.QueueStartupJump(0, 0, 0, false)
	if app.pendingInstanceJump != nil {
		t.Fatal("page 0 means no jump was requested")
	}
}

func TestApplyInstanceRequestRejectsUnknownCommands(t *testing.T) {
	app := testLayoutApp(3)
	err := app.applyInstanceRequest(instance.Request{Command: "explode"})
	if err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("err = %v, want one naming the unknown command", err)
	}
}

// Command-line coordinates are measured from the page's top-left corner, so a
// page whose box does not start at the origin must be shifted. Without this an
// external tool would have to know the crop offset, which it cannot.
func TestInstancePointIsRelativeToThePageCorner(t *testing.T) {
	app := testLayoutApp(3)
	app.pageMetrics[1].bounds = mupdf.Rect{X0: 10, Y0: 20, X1: 622, Y1: 812}

	x, y := app.pageRelativeToDocument(2, 100, 250)
	if x != 110 || y != 270 {
		t.Fatalf("got (%v,%v), want (110,270)", x, y)
	}
	// A page with a zero origin passes coordinates through untouched.
	x, y = app.pageRelativeToDocument(1, 100, 250)
	if x != 100 || y != 250 {
		t.Fatalf("got (%v,%v), want (100,250)", x, y)
	}
	// An out-of-range page must not panic; it simply cannot be shifted.
	if x, y = app.pageRelativeToDocument(99, 5, 6); x != 5 || y != 6 {
		t.Fatalf("got (%v,%v), want the input unchanged", x, y)
	}
}

func TestPollInstanceCommandsWithoutAServerIsHarmless(t *testing.T) {
	app := testLayoutApp(3)
	if app.pollInstanceCommands() {
		t.Fatal("no server means nothing to poll")
	}
}

// The address follows the open document, so switching files hands the old one
// back and claims the new one.
func TestInstanceAddressFollowsTheOpenDocument(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.pdf")
	second := filepath.Join(dir, "second.pdf")
	for _, path := range []string{first, second} {
		if err := os.WriteFile(path, []byte("%PDF"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	app := testLayoutApp(3)
	app.docPath = first
	app.EnableSingleInstance()
	if app.instanceServer == nil {
		t.Fatal("no listener was claimed for the first document")
	}
	firstAddress := app.instanceAddress
	wantFirst, err := instance.AddressFor(first)
	if err != nil {
		t.Fatal(err)
	}
	if firstAddress != wantFirst {
		t.Fatalf("address = %q, want %q", firstAddress, wantFirst)
	}
	t.Cleanup(app.CloseInstanceServer)

	// Rebinding for the same document must keep the existing listener.
	app.rebindInstanceServer()
	if app.instanceAddress != firstAddress {
		t.Fatal("rebinding for the same document changed the address")
	}

	app.docPath = second
	app.rebindInstanceServer()
	if app.instanceAddress == firstAddress {
		t.Fatal("the address did not follow the document")
	}
	if _, err := os.Stat(firstAddress); !os.IsNotExist(err) {
		t.Error("the old address should be released so another window can claim it")
	}
	if _, err := instance.Send(app.instanceAddress, instance.Request{Command: "ping"}); err != nil {
		t.Fatalf("the new address does not answer: %v", err)
	}
}

func TestSingleInstanceWithoutADocumentDoesNotListen(t *testing.T) {
	app := testLayoutApp(0)
	app.docPath = ""
	app.EnableSingleInstance()
	if app.instanceServer != nil {
		app.CloseInstanceServer()
		t.Fatal("a window with no document has nothing to be addressed by")
	}
}

// A command the viewer does not know must fail the caller, not exit cleanly.
// This also guards the message prefix the check depends on.
func TestInstanceRunReportsUnknownCommand(t *testing.T) {
	app := testLayoutApp(3)
	err := app.applyInstanceRun("definitely-not-a-command")
	if err == nil {
		t.Fatal("an unknown command reported success")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("err = %v, want it to name the unknown command", err)
	}
	// The dispatcher's wording is what the check keys on; if it changes, this
	// fails rather than silently letting typos succeed.
	if !strings.HasPrefix(app.message, unknownCommandPrefix) {
		t.Fatalf("viewer message = %q, want the %q prefix", app.message, unknownCommandPrefix)
	}
	if err := app.applyInstanceRun(""); err == nil {
		t.Error("an empty command should be rejected")
	}
}

// The window keeps its last message, so a failure must not be attributed to a
// later command, and a silent command must not wipe what is on screen.
func TestInstanceRunDoesNotInheritAStaleMessage(t *testing.T) {
	app := testLayoutApp(3)
	if err := app.applyInstanceRun("definitely-not-a-command"); err == nil {
		t.Fatal("the first unknown command should fail")
	}
	// A valid command must now succeed even though the failure is still shown.
	if err := app.applyInstanceRun("fit page"); err != nil {
		t.Fatalf("a valid command inherited the previous failure: %v", err)
	}
	// The same typo must fail again rather than being deduplicated.
	if err := app.applyInstanceRun("definitely-not-a-command"); err == nil {
		t.Fatal("a repeated unknown command should fail again")
	}

	// A command that reports nothing leaves the existing message alone.
	app.message = "something the user should still see"
	if err := app.applyInstanceRun("fit page"); err != nil {
		t.Fatal(err)
	}
	if app.message != "something the user should still see" {
		t.Fatalf("message = %q, want it preserved", app.message)
	}
}
