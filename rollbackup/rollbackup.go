package main

import (
	"errors"
	"fmt"
	"github.com/camlistore/lock"
	"github.com/codegangsta/cli"
	"github.com/rollbackup/agent/rolly"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var LockPath = "/var/lock/rollbackup.lock"
var Version = "dev"

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

	fmt.Printf("Restore backup %s to %s...\n", backupId, absPath)

	if err := a.Restore(backupId, absPath); err != nil {
		log.Fatal(err)
	}
}

func StatusAction(c *cli.Context) {
	a := getAgent(c)
	a.GetFolders()
}

func InitAction(c *cli.Context) {
	CheckUid(c.Command.Name)
	if err := registerHost(c); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Success! Host ready to backup.")
	fmt.Println("Add folder with `rollbackup add [<path>]`")
}

func registerHost(c *cli.Context) error {
	authData := c.Args().First()

	var hostId string
	var token string

	if strings.HasPrefix(authData, "u") {
		conn, err := rolly.NewBackend(c.GlobalString("backend"))
		if err != nil {
			return err
		}

		if hostAuth, err := rolly.InitHost(conn, authData, Version); err == nil {
			hostId = hostAuth.HostId
			token = hostAuth.Token
		} else {
			return err
		}

	} else {
		hostIdAndToken := strings.SplitN(authData, "@", 2)
		if len(hostIdAndToken) != 2 {
			return errors.New("Invalid auth token")
		}

		hostId = hostIdAndToken[0]
		token = hostIdAndToken[1]
	}

	// generate ssh key, if not exists
	if _, err := os.Stat(rolly.KeyPath); os.IsNotExist(err) {
		if err := rolly.GenerateClientKey(rolly.KeyPath); err != nil {
			return err
		}
	}

	publicKey, err := ioutil.ReadFile(rolly.PublicKeyPath())
	if err != nil {
		return err
	}

	if err := rolly.NewAgent(c.GlobalString("backend"), hostId, token, Version).Register(string(publicKey)); err != nil {
		return err
	}

	conf := &rolly.Config{HostId: hostId, Token: token, Version: Version}
	if err := rolly.WriteConfig(conf, rolly.ConfigPath()); err != nil {
		return err
	}

	return nil
}

func getAgent(c *cli.Context) *rolly.Agent {
	config, err := rolly.LoadConfig(rolly.ConfigPath())
	if err != nil {
		log.Fatal(err)
	}
	a := rolly.NewAgent(c.GlobalString("backend"), config.HostId, config.Token, Version)
	return a
}

func AddAction(c *cli.Context) {
	a := getAgent(c)

	if !c.Args().Present() {
		log.Fatal("Add folder with `rollbackup add [<path>]`")
	}

	// TODO: validate path
	absPath, err := filepath.Abs(c.Args().Get(0))
	if err != nil {
		log.Fatal(err)
	}

	if err := a.AddFolder(absPath); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Success! %s added.\n", absPath)
}

func BackupAction(c *cli.Context) {
	CheckUid(c.Command.Name)
	a := getAgent(c)

	if err := a.TrackMetrics(); err != nil {
		fmt.Println(err)
	}

	if _, err := lock.Lock(LockPath); err != nil {
		log.Fatal(err)
	}

	if err := a.RunTasks(); err != nil {
		log.Fatal(err)
	} else {
		fmt.Println("Completed!")
	}

}

func EnableAction(c *cli.Context) {
	CheckUid(c.Command.Name)

	if err := rolly.WriteCrontab(); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Success! Agent backup schedule runs with crontab.")
}

func DisableAction(c *cli.Context) {
	CheckUid(c.Command.Name)

	if err := rolly.RemoveCrontab(); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Success! Agent backup schedule disabled.")
}

func CheckUid(commandName string) {
	if os.Getuid() != 0 {
		fmt.Printf("FAILED! Are you root? Please, run `sudo rollbackup %s [ARGS]`\n", commandName)
		os.Exit(0)
	}
}

func main() {
	app := cli.NewApp()
	app.Version = Version
	app.Name = "rollbackup"
	app.Author = "RollBackup LLC"
	app.Email = "mail@rollbackup.com"
	app.Usage = "A client utility for manage backup via RollBackup.com"
	app.Flags = []cli.Flag{
		cli.StringFlag{"backend", "backend.rollbackup.com:8443", "command service backend"},
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
			Name:   "restore",
			Usage:  "Restore backup",
			Action: RestoreAction,
		},
		{
			Name:   "on",
			Usage:  "Enable periodic backup",
			Action: EnableAction,
		},
		{
			Name:   "off",
			Usage:  "Disable periodic backup",
			Action: DisableAction,
		},
	}

	app.Run(os.Args)
}
