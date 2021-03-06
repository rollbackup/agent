package rolly

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/rollbackup/gosigar"
	"github.com/rollbackup/rb"
	"github.com/rollbackup/secrpc"
	"io/ioutil"
	"log"
	"net/rpc"
	"net/rpc/jsonrpc"
	"os"
	"os/exec"
	"sync"
)

type TaskResult struct {
	Task   rb.Task
	Error  error
	Output string
}

type Job struct {
	Tasks []rb.Task
}

type Client struct {
	Url    string
	HostId string
	Token  string
}

func NewBackend(backendAddr string) (*rpc.Client, error) {
	conn, err := secrpc.SecureDial("tcp", backendAddr, []byte(RollbackupCA))
	if err != nil {
		log.Fatal("agent connection: ", err)
		return nil, err
	}
	return jsonrpc.NewClient(conn), nil
}

func InitHost(backend *rpc.Client, userToken string, agentVersion string) (hostAuth *rb.HostAuth, err error) {
	args := rb.HostInitParams{Token: userToken, AgentVersion: agentVersion}
	args.Hostname, err = os.Hostname()
	if err != nil {
		return
	}
	err = backend.Call("Host.Init", args, &hostAuth)
	return
}

func NewAgent(backendAddr, hostId, token, agentVersion string) *Agent {
	conn, err := secrpc.SecureDial("tcp", backendAddr, []byte(RollbackupCA))
	if err != nil {
		log.Fatal("agent connection: ", err)
	}

	return &Agent{
		auth:    &rb.HostAuth{HostId: hostId, Token: token, AgentVersion: agentVersion},
		backend: jsonrpc.NewClient(conn),
	}
}

type Agent struct {
	auth    *rb.HostAuth
	backend *rpc.Client
}

func (a *Agent) AddFolder(path string) error {
	args := rb.HostAddFolderParams{Auth: *a.auth, Path: path}
	var reply rb.HostOpResult
	return a.backend.Call("Host.AddFolder", args, &reply)

}

func (a *Agent) GetFolders() error {
	args := rb.HostGetFoldersParams{Auth: *a.auth}
	var reply rb.HostGetFoldersResult
	if err := a.backend.Call("Host.GetFolders", args, &reply); err != nil {
		return err
	}
	log.Println(reply)
	return nil
}

func (a *Agent) GetBackup(backupId string) error {
	args := rb.HostGetBackupParams{Auth: *a.auth, BackupId: backupId}
	var reply rb.HostGetBackupResult
	return a.backend.Call("Host.GetBackup", args, &reply)
}

func (a *Agent) Register(publicKey string) error {
	args := rb.HostRegisterParams{Auth: *a.auth, PublicKey: publicKey}
	var reply rb.HostOpResult
	return a.backend.Call("Host.Register", args, &reply)
}

func (a *Agent) TrackMetrics() error {
	params := rb.HostTrackMetricsParams{
		Auth: *a.auth,
	}

	var err error
	params.Hostname, err = os.Hostname()
	if err != nil {
		log.Println(err)
	}

	sig := sigar.ConcreteSigar{}

	params.LoadAverage, err = sig.GetLoadAverage()
	if err != nil {
		log.Println(err)
	}

	params.Mem, err = sig.GetMem()
	if err != nil {
		log.Println(err)
	}

	params.Swap, err = sig.GetSwap()
	if err != nil {
		log.Println(err)
	}

	params.Uptime = sigar.Uptime{}
	if params.Uptime.Get() != nil {
		log.Println(err)
	}

	params.FileSystemList = sigar.FileSystemList{}
	if params.FileSystemList.Get() != nil {
		log.Println(err)
	}

	params.FileSystemUsage = make(map[string]sigar.FileSystemUsage)
	for _, fs := range params.FileSystemList.List {
		if fs.DirName == "" {
			continue
		}

		usage := sigar.FileSystemUsage{}
		if err := usage.Get(fs.DirName); err == nil {
			params.FileSystemUsage[fs.DirName] = usage
		} else {
			log.Println(err)
		}
	}

	params.CpuList = sigar.CpuList{}
	if params.CpuList.Get() != nil {
		log.Println(err)
	}

	params.NetworkUtilization = sigar.NetworkUtilization{}
	if params.NetworkUtilization.Get() != nil {
		log.Println(err)
	}

	if params.DiskStats, err = sigar.GetDiskStats(); err != nil {
		log.Println(err)
	}

	var reply rb.HostOpResult
	return a.backend.Call("Host.TrackMetrics", params, &reply)
}

func runBackupPlugin(t *rb.Task) error {
	p := &Plugin{Name: t.Plugin.Name, Version: t.Plugin.Version}
	if !p.Exists() {
		if err := p.Download(); err != nil {
			return err
		}
	}

	if err := p.Backup(t.Local, t.Plugin.Options); err != nil {
		return err
	}
	return nil
}

func (a *Agent) RunTasks() error {
	args := rb.HostGetTasksParams{Auth: *a.auth}
	var reply rb.HostGetTasksResult
	err := a.backend.Call("Host.GetTasks", args, &reply)
	if err != nil {
		return err
	}

	if !reply.Success && len(reply.Tasks) == 0 {
		return nil
	}

	var wg sync.WaitGroup
	for _, t := range reply.Tasks {
		wg.Add(1)
		go func(t rb.Task) {
			defer wg.Done()

			if t.Plugin.Name != "" {
				if outdir, err := ioutil.TempDir("", "rollbackup_plugin_"+t.Plugin.Name); err != nil {
					log.Println(err)
					return
				} else {
					t.Local = outdir + "/"
				}

				if err := runBackupPlugin(&t); err != nil {
					log.Println(err)
					return
				}
			}

			if _, err := os.Stat(t.Local); err != nil {
				log.Println(err)
				a.logBackup(&rb.HostLogBackupParams{
					BackupId:  t.BackupId,
					FolderId:  t.FolderId,
					Path:      t.Local,
					StatError: fmt.Sprintf("%s", err),
				})
				return
			}

			log.Printf("Start backup %s...", t.Local)
			if out, err := a.backup(&t); err == nil {
				if err := a.commitBackup(&t, out); err != nil {
					log.Printf("Commit Backup Error: %s", err)
				}
			} else {
				log.Printf("Fail Backup %s error: %s", t.Local, err)
			}
		}(t)
	}
	wg.Wait()

	return nil
}

func (a *Agent) logBackup(log *rb.HostLogBackupParams) error {
	log.Auth = *a.auth
	var reply rb.HostOpResult
	return a.backend.Call("Host.LogBackup", log, &reply)
}

func (a *Agent) backup(t *rb.Task) (string, error) {
	fpFile, err := makeKnownHosts(t.SshFingerprint)
	if err != nil {
		log.Println(err)
		return "", err
	}
	defer os.Remove(fpFile)

	args := buildRsyncArgs(fpFile, KeyPath)
	args = append(args, t.Args...)
	args = append(args, "--stats", t.Local, t.Remote)
	log.Print("RSYNC: ", args)

	cmd := exec.Command("rsync", args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd.Stderr = &stderr
	cmd.Stdout = &stdout

	err = cmd.Run()

	if err != nil {
		log.Println(stderr.String())
	}

	a.logBackup(&rb.HostLogBackupParams{
		BackupId:    t.BackupId,
		FolderId:    t.FolderId,
		Path:        t.Local,
		RsyncArgs:   args,
		RsyncStdout: stdout.String(),
		RsyncStderr: stderr.String(),
		ExecError:   fmt.Sprintf("%s", err),
	})

	return stdout.String(), err
}

func (a *Agent) commitBackup(task *rb.Task, rsyncOutput string) error {
	args := rb.HostCommitBackupParams{
		Auth:        *a.auth,
		FolderId:    task.FolderId,
		BackupId:    task.BackupId,
		RsyncOutput: rsyncOutput,
	}
	var reply rb.HostOpResult
	return a.backend.Call("Host.CommitBackup", args, &reply)
}

func makeKnownHosts(sshFp string) (string, error) {
	f, err := ioutil.TempFile("", "rollbackup_known_hosts")
	if err != nil {
		return "", err
	}

	f.WriteString(sshFp)
	return f.Name(), nil
}

func buildRsyncArgs(sshFp, sshKey string) []string {
	return []string{"-az", "-e", fmt.Sprintf("ssh -c arcfour -o Compression=no -o StrictHostKeyChecking=yes -o UserKnownHostsFile=%s -i %s", sshFp, sshKey)}
}

func (a *Agent) RestorePlugin(pluginName string, backupId string) error {
	return nil
}

func (a *Agent) Restore(backupId string, dest string) error {
	args := rb.HostGetBackupParams{Auth: *a.auth, BackupId: backupId}
	var reply rb.HostGetBackupResult
	err := a.backend.Call("Host.GetBackup", args, &reply)
	if err != nil {
		return err
	}

	if !reply.Success {
		return errors.New("backup not found")
	}

	if reply.Plugin.Name != "" && dest == "" {
		dest, err = ioutil.TempDir("", "rollbackup_plugin_restore_"+reply.Plugin.Name)
		if err != nil {
			return err
		}
	}

	if _, err := os.Stat(dest); err == nil {
		// TODO: add prompt and cmd-flag for force
		return fmt.Errorf("directory already exists: %s", dest)
	}

	err = runRestore(dest, reply.RsyncUrl+"/", reply.SshFingerprint)
	if err != nil {
		return err
	}

	if reply.Plugin.Name == "" {
		return nil
	}
	log.Printf("Restore with %s (v%s)", reply.Plugin.Name, reply.Plugin.Version)

	p := NewPlugin(reply.Plugin.Name, reply.Plugin.Version)
	if !p.Exists() {
		if err := p.Download(); err != nil {
			return err
		}
	}

	if err := p.Restore(dest, reply.Plugin.Options); err != nil {
		return err
	}

	return nil
}

func runRestore(local, remote, sshFp string) error {
	fpFile, err := makeKnownHosts(sshFp)
	if err != nil {
		return err
	}
	defer os.Remove(fpFile)

	args := buildRsyncArgs(fpFile, KeyPath)
	args = append(args, remote, local)

	cmd := exec.Command("rsync", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return err
	}

	log.Printf("Copy from remote host...")
	if err := cmd.Wait(); err != nil {
		log.Printf("Command finished with error: %v", err)
		return err
	}
	log.Printf("OK. Completed!")

	return nil
}
