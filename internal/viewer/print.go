package viewer

import (
	"os/exec"
	"runtime"
	"strings"
)

// printDocument sends the document file to the system's print service.
// args are passed to lp, for example "-d office -P 1-3".
func (a *App) printDocument(args string) {
	if a.docPath == "" {
		a.message = "no document open"
		return
	}
	cmd := printCommand(a.docPath, strings.Fields(args))
	if err := cmd.Start(); err != nil {
		a.message = "print: " + err.Error()
		return
	}
	go cmd.Wait()
	a.message = "sent " + a.docName + " to the printer"
	if a.unsaved {
		a.message += " (without unsaved edits)"
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
