package main

import (
	"flag"
	"fmt"
	"github.com/camlistore/lock"
	"github.com/rollbackup/agent/rolly"
	"log"
	"os"
	"path/filepath"
)

func main() {
	flag.Parse()

	if flag.Arg(0) == "init" {
		if err := rolly.Bootstrap(flag.Arg(1)); err != nil {
			log.Fatal(err)
		}
		fmt.Println("Success! Host ready to backup.")
		return
	}

	config, err := rolly.LoadConfig(rolly.ConfigPath)
	if err != nil {
		log.Fatal(err)
	}

	a := rolly.NewAgent(config.HostId, config.Token)

	switch flag.Arg(0) {
	case "add":
		if flag.NArg() != 2 {
			log.Fatal("no path")
		}

		// TODO: validate path
		absPath, err := filepath.Abs(flag.Arg(1))
		if err != nil {
			log.Fatal(err)
		}
		if err := a.AddFolder(absPath); err != nil {
			log.Fatal(err)
		}
	case "backup":
		// get exclusive lock
		lockfile := filepath.Join(os.TempDir(), "rollbackup.lock")
		if _, err := lock.Lock(lockfile); err != nil {
			log.Fatal(err)
		}
		if err := a.RunTasks(); err != nil {
			log.Fatal(err)
		}
		log.Println("ok")
	default:
		fmt.Println("unknown arg")
		os.Exit(1)
	}
}
