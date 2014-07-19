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

func RestoreAction(c *cli.Context) {
	a := getAgent(c)
	backupId := c.Args().Get(0)
	destPath := c.Args().Get(1)

	if destPath == "" {
		destPath = "."
	}

	absPath, err := filepath.Abs(destPath)
	if err != nil {
		log.Fatal(err)
	}

	if backupId == "" {
		log.Fatal("no params")
	}

	log.Printf("Restore backup %s to %s...\n", backupId, absPath)

	if err := a.Restore(backupId, absPath); err != nil {
		log.Fatal(err)
	}
}

func StatusAction(c *cli.Context) {
	a := getAgent(c)
	a.GetFolders()
}

func InitAction(c *cli.Context) {
	if err := rolly.Bootstrap(c.GlobalString("backend"), c.Args().First()); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Success! Host ready to backup.")
	fmt.Println("Add folder with `rollbackup add [<path>]`")
}

func getAgent(c *cli.Context) *rolly.Agent {
	config, err := rolly.LoadConfig(rolly.ConfigPath())
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

	log.Println("completed")
}

func EnableAction(c *cli.Context) {
	if err := rolly.WriteCrontab(); err != nil {
		log.Fatal(err)
	}

	log.Println("Success! Agent backup schedule runs with crontab.")
}

func DisableAction(c *cli.Context) {
	if err := rolly.RemoveCrontab(); err != nil {
		log.Fatal(err)
	}

	log.Println("Success! Agent backup schedule disabled.")
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
		{
			Name:   "status",
			Usage:  "Show folders backup status",
			Action: StatusAction,
		},
		{
			Name:   "restore",
			Usage:  "Restore backup",
			Action: RestoreAction,
		},
		{
			Name:   "on",
			Usage:  "Enable",
			Action: EnableAction,
		},
		{
			Name:   "off",
			Usage:  "Disable",
			Action: DisableAction,
		},
	}

	app.Run(os.Args)
}
