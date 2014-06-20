package rolly

import (
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	// "path/filepath"
	// "log"
	"strings"
)

const KeyPath = "/tmp/rbdkey"

func PublicKeyPath() string {
	return KeyPath + ".pub"
}

func Bootstrap(backendAddr, authdata string) error {
	hostIdAndToken := strings.SplitN(authdata, "@", 2)
	if len(hostIdAndToken) != 2 {
		return errors.New("Unable to parse token")
	}

	hostId := hostIdAndToken[0]
	token := hostIdAndToken[1]

	// generate ssh key, if not exists
	if _, err := os.Stat(KeyPath); os.IsNotExist(err) {
		if err := generateClientKey(KeyPath); err != nil {
			return err
		}
	}

	publicKey, err := ioutil.ReadFile(PublicKeyPath())
	if err != nil {
		return err
	}

	if err := NewAgent(backendAddr, hostId, token).Register(string(publicKey)); err != nil {
		return err
	}

	c := &Config{HostId: hostId, Token: token}
	if err := WriteConfig(c, ConfigPath); err != nil {
		return err
	}

	return nil
}

func generateClientKey(path string) error {
	// TODO: add restrict chmod
	args := []string{"-b", "2048", "-t", "rsa", "-f", path, "-N", "", "-q"}
	cmd := exec.Command("ssh-keygen", args...)
	if err := cmd.Run(); err != nil {
		return err
	}

	return os.Chmod(path, 0600)
}
