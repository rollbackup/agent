package rolly

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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
	return filepath.Join(p.Dir(), "backup.sh")
}

func (p *Plugin) Dir() string {
	return filepath.Join(PluginBase, p.Name+"-"+p.Version)
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
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Plugin download HTTP-error: %d", resp.StatusCode)
	}

	log.Printf("Status %s", resp.StatusCode)
	defer resp.Body.Close()
	fpath := filepath.Join(PluginBase, fname)

	out, err := os.Create(fpath)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		return err
	}

	log.Printf("Plugin %s saved to %s", p.Name, fpath)
	log.Printf("Extract %s to %s", fpath, p.Dir())
	if err := unzip(fpath, PluginBase); err != nil {
		log.Printf("unzip error: %s\n", err)
		return err
	}

	return nil
}

func (p *Plugin) Exists() bool {
	if _, err := os.Stat(p.Dir()); err != nil {
		return false
	}
	return true
}

func unzip(src, dest string) error {
	//TODO: may be lag in large archives, because stack grow with defer each file close
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()

		path := filepath.Join(dest, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(path, f.Mode())
		} else {
			f, err := os.OpenFile(
				path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer f.Close()

			_, err = io.Copy(f, rc)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
