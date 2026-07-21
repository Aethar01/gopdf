package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	_ "embed"

	"gopdf/internal/config"
	"gopdf/internal/viewer"
)

var version = "0.1.13"

//go:embed assets/gopdf.bmp
var iconBMP []byte

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	var cfgPath string
	var startPage int
	var printVersion bool
	var verbose bool
	flag.StringVar(&cfgPath, "config", "", "path to config.lua")
	flag.IntVar(&startPage, "page", 1, "1-based page to open")
	flag.BoolVar(&printVersion, "v", false, "print version")
	flag.BoolVar(&verbose, "V", false, "enable verbose logging")
	flag.Parse()
	pageSet := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "page" {
			pageSet = true
		}
	})

	if printVersion {
		fmt.Println(version)
		return nil
	}

	var docPath string
	if flag.NArg() == 0 {
		if recent := config.RecentFiles(1); len(recent) > 0 {
			docPath = recent[0]
			if session, ok := config.GetDocumentSession(docPath); !pageSet && ok {
				startPage = session.Page + 1
			}
		}
	} else {
		var err error
		docPath, err = explicitDocumentPath(flag.Arg(0))
		if err != nil {
			return err
		}
	}
	if verbose {
		log.Printf("verbose logging enabled")
		log.Printf("startup config=%q page=%d doc=%q", cfgPath, startPage, docPath)
	}

	runtime, err := config.Open(cfgPath, docPath, verbose)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	defer runtime.Close()

	app, err := viewer.New(docPath, runtime, startPage-1, iconBMP, viewer.NewOptions{Verbose: verbose, StartPageExplicit: pageSet})
	if err != nil {
		return fmt.Errorf("start viewer: %w", err)
	}
	defer app.Close()

	return app.Run()
}

func explicitDocumentPath(path string) (string, error) {
	path = config.AbsoluteDocumentPath(path)
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("open document: %w", err)
	}
	return path, nil
}
