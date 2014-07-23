package rolly

import (
	"os"
	"os/exec"
)

const KeyPath = "/etc/rollbackup/id_rsa"

func PublicKeyPath() string {
	return KeyPath + ".pub"
}

func GenerateClientKey(path string) error {
	// TODO: add restrict chmod
	args := []string{"-b", "2048", "-t", "rsa", "-f", path, "-N", "", "-q"}
	cmd := exec.Command("ssh-keygen", args...)
	if err := cmd.Run(); err != nil {
		return err
	}

	return os.Chmod(path, 0644)
}
