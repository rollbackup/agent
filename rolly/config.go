package rolly

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os/user"
	"path/filepath"
)

type Config struct {
	Token  string
	HostId string
}

func ConfigPath() string {
	if u, err := user.Current(); err == nil {
		return filepath.Join(u.HomeDir, ".rollbackup.conf")
	} else {
		panic(err)
	}
}

func LoadConfig(filename string) (*Config, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func WriteConfig(c *Config, configPath string) error {
	if data, err := json.Marshal(c); err == nil {
		if err := ioutil.WriteFile(configPath, data, 0644); err != nil {
			return err
		}
	} else {
		return err
	}
	return nil
}

func WriteCrontab(token string) error {
	cmd := fmt.Sprintf("* * * * * /usr/bin/rollbackup backup %s", token)
	if err := ioutil.WriteFile("/etc/cron.d/rollbackup", []byte(cmd), 0644); err != nil {
		return err
	}
	return nil
}
