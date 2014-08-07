package rolly

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
)

var PluginBase = "/tmp/plugin"

type Plugin struct {
	Name string
}

func (p *Plugin) BackupScript() string {
	return path.Join(p.Dir(), "backup.sh")
}

func (p *Plugin) Dir() string {
	return path.Join(PluginBase, p.Name)
}

func RunPlugin(p *Plugin, params map[string]string) error {
	cmd := exec.Command("bash", p.BackupScript())
	env := []string{}

	for k, v := range params {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	var err error
	if cmd.Dir, err = ioutil.TempDir("", "rollbackup_plugin_"+p.Name); err != nil {
		return err
	}

	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Start()

	if err != nil {
		return err
	}

	log.Printf("Waiting for command to finish...")
	if err = cmd.Wait(); err != nil {
		return err
	}
	log.Printf("Command finished with error: %v", err)

	return nil
}

func DownloadPlugin(p *Plugin) error {
	fname := p.Name + ".zip"
	resp, err := http.Get("http://dist.rollbackup.com/plugin/" + fname)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	out, err := os.Create(path.Join(PluginBase, fname))
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		return err
	}

	cmd := exec.Command("unzip", path.Join(PluginBase, fname), "-d", p.Dir())
	err = cmd.Run()
	if err != nil {
		log.Printf("Command finished with error: %v", err)
		return err
	}

	return nil
}
