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
	runtime, err := config.Open(cfgPath, docPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	defer runtime.Close()

	app, err := viewer.New(docPath, runtime, startPage-1)
	if err != nil {
		log.Fatalf("start viewer: %v", err)
	}
	defer app.Close()

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
