package main

import (
	"flag"
	"fmt"
	"log"

	"gopdf/internal/config"
	"gopdf/internal/viewer"
)

const version = "0.1.1"

func main() {
	var cfgPath string
	var startPage int
	var printVersion bool
	flag.StringVar(&cfgPath, "config", "", "path to config.lua")
	flag.IntVar(&startPage, "page", 1, "1-based page to open")
	flag.BoolVar(&printVersion, "v", false, "print version")
	flag.Parse()

	if printVersion {
		fmt.Println(version)
		return
	}

	var docPath string
	if flag.NArg() == 0 {
		docPath = config.GetLastFile()
	} else {
		docPath = flag.Arg(0)
	}

	for {
		runtime, err := config.Open(cfgPath, docPath)
		if err != nil {
			log.Fatalf("load config: %v", err)
		}

		app, err := viewer.New(docPath, runtime, startPage-1)
		if err != nil {
			runtime.Close()
			log.Fatalf("start viewer: %v", err)
		}

		if err := app.Run(); err != nil {
			app.Close()
			runtime.Close()
			log.Fatal(err)
		}

		newPath := app.PendingOpen()
		app.Close()
		runtime.Close()

		if newPath == "" {
			break
		}
		docPath = newPath
		startPage = 1
	}
}
