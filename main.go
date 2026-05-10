package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"gopdf/internal/config"
	"gopdf/internal/viewer"
)

func main() {
	var cfgPath string
	var startPage int
	flag.StringVar(&cfgPath, "config", "", "path to config.lua")
	flag.IntVar(&startPage, "page", 1, "1-based page to open")
	flag.Parse()

	if flag.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "usage: %s [--config path] [--page N] <file.pdf>\n", os.Args[0])
		os.Exit(2)
	}

	docPath := flag.Arg(0)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	app, err := viewer.New(docPath, cfg, startPage-1)
	if err != nil {
		log.Fatalf("start viewer: %v", err)
	}
	defer app.Close()

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
