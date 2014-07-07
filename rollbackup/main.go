package main

import (
	"fmt"
	"github.com/camlistore/lock"
	"github.com/codegangsta/cli"
	"github.com/rollbackup/agent/rolly"
	"log"
	"os"
	"path/filepath"
)

func InitAction(c *cli.Context) {
	if err := rolly.Bootstrap(c.GlobalString("backend"), c.Args().First()); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Success! Host ready to backup.")
}

func getAgent(c *cli.Context) *rolly.Agent {
	config, err := rolly.LoadConfig(rolly.ConfigPath)
	if err != nil {
		log.Fatal(err)
	}
	a := rolly.NewAgent(c.GlobalString("backend"), config.HostId, config.Token)
	return a
}

func AddAction(c *cli.Context) {
	a := getAgent(c)

	if !c.Args().Present() {
		log.Fatal("no path")
	}

	// TODO: validate path
	absPath, err := filepath.Abs(c.Args().Get(0))
	if err != nil {
		log.Fatal(err)
	}

	if err := a.AddFolder(absPath); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s added\n", absPath)
}

func BackupAction(c *cli.Context) {
	a := getAgent(c)

	lockfile := filepath.Join(os.TempDir(), "rollbackup.lock")
	if _, err := lock.Lock(lockfile); err != nil {
		log.Fatal(err)
	}

	if err := a.RunTasks(); err != nil {
		log.Fatal(err)
	}

	log.Println("ok")
}

func main() {
	app := cli.NewApp()
	app.Name = "rollbackup"
	app.Usage = "A client utility for manage backup via RollBackup.com"
	app.Flags = []cli.Flag{
		cli.StringFlag{"backend,b", "backend.rollbackup.com:8443", "service backend endpoint"},
	}

	app.Commands = []cli.Command{
		{
			Name:   "init",
			Usage:  "Configure agent with signed token",
			Action: InitAction,
		},
		{
			Name:   "add",
			Usage:  "Add folder to periodic backup",
			Action: AddAction,
		},
		{
			Name:   "backup",
			Usage:  "Make a backup",
			Action: BackupAction,
		},
	}

	app.Run(os.Args)
}
