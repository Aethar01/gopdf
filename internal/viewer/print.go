package viewer

import (
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// Printing goes through CUPS' lp on Linux and macOS, from a small dialog
// for the printer, pages, copies and sides, or straight from :print with lp
// options. Windows hands the file to the shell's print verb.

type printSettings struct {
	printer string
	pages   string // lp page ranges such as "1-3,5"; empty prints all
	copies  int
	sides   string // lp sides value; empty prints one-sided
}

var printSides = []struct{ value, label string }{
	{"", "off"},
	{"two-sided-long-edge", "long edge"},
	{"two-sided-short-edge", "short edge"},
}

func (s printSettings) lpArgs() []string {
	var args []string
	if s.printer != "" {
		args = append(args, "-d", s.printer)
	}
	if s.pages != "" {
		args = append(args, "-P", s.pages)
	}
	if s.copies > 1 {
		args = append(args, "-n", strconv.Itoa(s.copies))
	}
	if s.sides != "" {
		args = append(args, "-o", "sides="+s.sides)
	}
	return args
}

// printDocument prints the document file: with lp options given, at once,
// and otherwise through the print dialog.
func (a *App) printDocument(args string) {
	if a.docPath == "" {
		a.message = "no document open"
		return
	}
	if runtime.GOOS == "windows" || strings.TrimSpace(args) != "" {
		a.runPrint(strings.Fields(args))
		return
	}
	a.findPrinters()
	a.showPrintDialog()
}

// printerList is what lpstat reported: the printers and CUPS' default.
type printerList struct {
	names       []string
	defaultName string
}

// findPrinters asks CUPS for printers in the background, as lpstat can take
// a second while it looks for network printers.
func (a *App) findPrinters() {
	found := make(chan printerList, 1)
	a.printersFound = found
	go func() {
		names := listPrinters()
		found <- printerList{names: names, defaultName: defaultPrinter(names)}
	}()
}

func (a *App) showPrintDialog() {
	s := &a.printSettings
	s.copies = max(1, s.copies)
	sides := "off"
	for _, side := range printSides {
		if side.value == s.sides {
			sides = side.label
		}
	}
	printer := s.printer
	switch {
	case printer == "" && a.printersFound != nil:
		printer = "looking…"
	case printer == "":
		printer = "none found"
	}
	pages := s.pages
	if pages == "" {
		pages = "all"
	}
	rows := []uiRow{
		{text: "Printer", secondary: printer, value: "printer"},
		{text: "Pages", secondary: pages, value: "pages"},
		{text: "Copies", secondary: strconv.Itoa(s.copies), value: "copies"},
		{text: "Two-sided", secondary: sides, value: "sides"},
		{text: "Print", value: "print"},
	}
	for i := range rows {
		rows[i].index = i
	}
	view := a.showRowList("print", "Print "+a.docName, rows, 50, 40)
	view.searchable = false
	view.onSelect = func(a *App, row uiRow) {
		switch row.value {
		case "printer":
			a.pickPrinter(a.printers.names)
		case "pages":
			a.askPrompt("Pages (e.g. 1-3,5; blank for all)", s.pages, func(v string) {
				s.pages = strings.ReplaceAll(v, " ", "")
				a.showPrintDialog()
			})
		case "copies":
			a.askPrompt("Copies", strconv.Itoa(s.copies), func(v string) {
				if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n >= 1 {
					s.copies = n
				}
				a.showPrintDialog()
			})
		case "sides":
			for i, side := range printSides {
				if side.value == s.sides {
					s.sides = printSides[(i+1)%len(printSides)].value
					break
				}
			}
			a.showPrintDialog()
			a.activeUIView().selected = row.index
		case "print":
			a.closeAllUI()
			a.runPrint(s.lpArgs())
		}
	}
}

func (a *App) pickPrinter(printers []string) {
	if len(printers) == 0 {
		a.message = "no printers found (lpstat -e)"
		return
	}
	rows := make([]uiRow, len(printers))
	for i, p := range printers {
		rows[i] = uiRow{index: i, text: p, value: p}
	}
	view := a.showRowList("printers", "Printer", rows, 40, 40)
	view.onSelect = func(a *App, row uiRow) {
		a.printSettings.printer = row.value
		a.showPrintDialog()
	}
}

// runPrint starts printing in the background; pollPrintResult reports the
// outcome.
func (a *App) runPrint(args []string) {
	cmd := printCommand(a.docPath, args)
	note := ""
	if a.unsaved {
		note = " (without unsaved edits)"
	}
	a.message = "printing " + a.docName + note
	results := make(chan string, 1)
	a.printResult = results
	go func() {
		out, err := cmd.CombinedOutput()
		msg := strings.TrimSpace(string(out))
		switch {
		case err != nil && msg != "":
			msg = "print failed: " + msg
		case err != nil:
			msg = "print failed: " + err.Error()
		case msg == "":
			msg = "sent " + a.docName + " to the printer"
		}
		results <- msg + note
	}()
}

// pollPrintResult reports a finished print and fills in found printers.
func (a *App) pollPrintResult() {
	select {
	case msg := <-a.printResult:
		a.message = msg
		a.printResult = nil
		a.pendingRedraw = true
	default:
	}
	select {
	case found := <-a.printersFound:
		a.printersFound = nil
		a.printers = found
		if a.printSettings.printer == "" {
			a.printSettings.printer = found.defaultName
		}
		if view := a.activeUIView(); view != nil && view.id == "print" {
			selected := view.selected
			a.showPrintDialog()
			a.activeUIView().selected = selected
		}
		a.pendingRedraw = true
	default:
	}
}

func printCommand(path string, args []string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("powershell", "-NoProfile", "-Command", "Start-Process", "-FilePath", powershellQuote(path), "-Verb", "Print")
	}
	return exec.Command("lp", append(args, "--", path)...)
}

func powershellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func listPrinters() []string {
	out, err := exec.Command("lpstat", "-e").Output()
	if err != nil {
		return nil
	}
	return strings.Fields(string(out))
}

// defaultPrinter is CUPS' default destination, or the first printer.
func defaultPrinter(printers []string) string {
	out, _ := exec.Command("lpstat", "-d").Output()
	if name, ok := parseDefaultPrinter(string(out)); ok {
		return name
	}
	if len(printers) > 0 {
		return printers[0]
	}
	return ""
}

func parseDefaultPrinter(lpstat string) (string, bool) {
	_, name, ok := strings.Cut(strings.TrimSpace(lpstat), "system default destination: ")
	return strings.TrimSpace(name), ok && name != ""
}
