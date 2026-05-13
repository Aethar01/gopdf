package main

import (
	"flag"
	"fmt"
	"log"

	_ "embed"

	"gopdf/internal/config"
	"gopdf/internal/viewer"
)

const version = "0.1.13"

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
	flag.StringVar(&cfgPath, "config", "", "path to config.lua")
	flag.IntVar(&startPage, "page", 1, "1-based page to open")
	flag.BoolVar(&printVersion, "v", false, "print version")
	flag.Parse()

	if printVersion {
		fmt.Println(version)
		return nil
	}

	var docPath string
	if flag.NArg() == 0 {
		docPath = config.GetLastFile()
	} else {
		docPath = flag.Arg(0)
	}

	runtime, err := config.Open(cfgPath, docPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	defer runtime.Close()

	app, err := viewer.New(docPath, runtime, startPage-1, iconBMP)
	if err != nil {
		return fmt.Errorf("start viewer: %w", err)
	}
	defer app.Close()

	return app.Run()
}
