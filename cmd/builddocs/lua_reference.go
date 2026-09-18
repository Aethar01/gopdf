package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"

	"gopdf/internal/config"
)

// loadLuaFunctionDocs reads ordinary Go doc comments from the named Lua
// handlers in internal/config. Handler names are derived from their Lua
// signatures, so the generated reference has one source of truth: the
// documented implementation in Go source, not a second metadata table.
func loadLuaFunctionDocs() (map[string]string, error) {
	files, err := parser.ParseDir(token.NewFileSet(), "internal/config", func(info os.FileInfo) bool {
		return !strings.HasSuffix(info.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse config documentation: %w", err)
	}
	pkg, ok := files["config"]
	if !ok {
		return nil, fmt.Errorf("config package not found while loading Lua documentation")
	}
	docs := map[string]string{}
	for _, file := range pkg.Files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Doc == nil || len(fn.Doc.List) == 0 || !strings.HasPrefix(fn.Name.Name, "lua") {
				continue
			}
			docs[fn.Name.Name] = strings.TrimSpace(fn.Doc.Text())
		}
	}
	return docs, nil
}

func luaDocumentationName(signature string) string {
	name, _, _ := strings.Cut(signature, "(")
	parts := strings.Split(name, ".")
	var b strings.Builder
	b.WriteString("lua")
	for _, part := range parts[1:] {
		for _, word := range strings.FieldsFunc(part, func(r rune) bool { return r == '_' || r == '-' }) {
			if word == "" {
				continue
			}
			runes := []rune(word)
			runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
			b.WriteString(string(runes))
		}
	}
	return b.String()
}

func validateLuaFunctionDocs(refs []config.LuaReferenceEntry, docs map[string]string) error {
	for _, ref := range refs {
		name := luaDocumentationName(ref.Signature)
		if strings.TrimSpace(docs[name]) == "" {
			return fmt.Errorf("missing Go documentation for Lua API %s (expected %s)", ref.Signature, name)
		}
	}
	return nil
}

func docSummary(signature, doc string) string {
	doc = formatLuaDoc(signature, doc)
	if paragraph, _, ok := strings.Cut(strings.TrimSpace(doc), "\n\n"); ok {
		return strings.ReplaceAll(paragraph, "\n", " ")
	}
	return strings.ReplaceAll(strings.TrimSpace(doc), "\n", " ")
}

func formatLuaDoc(signature, doc string) string {
	doc = strings.TrimSpace(doc)
	handler := luaDocumentationName(signature)
	doc = strings.TrimPrefix(doc, handler+" ")
	lines := strings.Split(doc, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "# ") {
			lines[i] = "#### " + strings.TrimPrefix(line, "# ")
		}
	}
	return strings.Join(lines, "\n")
}
