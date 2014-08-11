package rolly

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"
)

var PluginBase = "/usr/share/rollbackup"

func PluginUrl() string {
	url := os.Getenv("RB_PLUGIN_URL")
	if url == "" {
		url = "https://rollbackup.com"
	}
	return url
}

type Plugin struct {
	Name    string
	Version string
}

func (p *Plugin) BackupScript() string {
	return path.Join(p.Dir(), "backup.sh")
}

func (p *Plugin) Dir() string {
	return path.Join(PluginBase, p.Name+"-"+p.Version)
}

func (p *Plugin) Run(outpath string, params map[string]string) error {
	cmd := exec.Command("bash", p.BackupScript())
	env := []string{}

	for k, v := range params {
		env = append(env, fmt.Sprintf("RB_%s=%s", strings.ToUpper(k), v))
	}

	log.Printf("Plugin Env: %+v\n", env)

	cmd.Dir = outpath
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	if err != nil {
		return err
	}

	if err = cmd.Wait(); err != nil {
		log.Printf("Command finished with error: %v", err)
		return err
	}

	return nil
}

func (p *Plugin) Download() error {
	fname := fmt.Sprintf("%s_%s.zip", p.Name, p.Version)

	url := fmt.Sprintf("%s/plugin/%s/%s/download", PluginUrl(), p.Name, p.Version)

	log.Printf("Download plugin %s from %s", p.Name, url)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	fpath := path.Join(PluginBase, fname)

	out, err := os.Create(fpath)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		return err
	}
	log.Printf("Plugin %s saved to %s", p.Name, fpath)

	cmd := exec.Command("unzip", fpath, "-d", PluginBase)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	log.Printf("Extract to %s", p.Dir())
	err = cmd.Run()
	if err != nil {
		log.Printf("Command finished with error: %v", err)
		return err
	}

	return nil
}

func (p *Plugin) IsExists() bool {
	log.Printf("IsPluginExists: check %s", p.Dir())
	if _, err := os.Stat(p.Dir()); err != nil {
		return false
	}
	return true
}
