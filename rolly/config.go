package rolly

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

type Config struct {
	Token   string
	HostId  string
	Version string
}

func ConfigPath() string {
	return "/etc/rollbackup/agent.conf"
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

func WriteCrontab() error {
	cmd := fmt.Sprintf("* * * * * root /usr/local/bin/rollbackup backup >> /var/log/rollbackup_cron.log 2>&1\n\n")
	return ioutil.WriteFile("/etc/cron.d/rollbackup", []byte(cmd), 0600)
}

func RemoveCrontab() error {
	if err := os.Remove("/etc/cron.d/rollbackup"); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return nil
}
