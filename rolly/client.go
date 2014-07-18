package rolly

import (
	"bytes"
	"errors"
	"fmt"
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

func NewAgent(backendAddr, hostId, token string) *Agent {
	conn, err := secrpc.SecureDial("tcp", backendAddr, []byte(RollbackupCA))
	if err != nil {
		log.Fatal("agent connection: ", err)
	}
	return &Agent{
		auth:    &rb.HostAuth{HostId: hostId, Token: token},
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

	//log.Printf("%+v", reply.Tasks)
	var wg sync.WaitGroup
	for _, t := range reply.Tasks {
		wg.Add(1)
		go func(t *rb.Task) {
			log.Printf("Start backup %s...", t.Local)
			out, err := a.backup(t)
			if err != nil {
				log.Printf("Fail Backup %s error: %s", t.Local, err)
			}
			a.commitBackup(t, out, fmt.Sprintf("%s", err))
			wg.Done()
		}(&t)
	}
	wg.Wait()

	return nil
}

func (a *Agent) backup(task *rb.Task) (string, error) {
	fpFile, err := makeKnownHosts(task.SshFingerprint)
	if err != nil {
		log.Println(err)
		return "", err
	}
	defer os.Remove(fpFile)

	args := buildRsyncArgs(fpFile, KeyPath)
	if task.LinkDest != "" {
		args = append(args, fmt.Sprintf("--link-dest=%s", task.LinkDest))
	}
	args = append(args, "--stats", task.Local, task.Remote)

	// TODO: verbose logging
	//log.Println(args)
	cmd := exec.Command("rsync", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err = cmd.Run()
	return out.String(), err
}

func (a *Agent) commitBackup(task *rb.Task, output string, execErr string) error {
	args := rb.HostCommitBackupParams{
		Auth: *a.auth,
		FolderId: task.FolderId,
		BackupId: task.BackupId,
		RsyncOutput: output,
		RsyncExecError: execErr,
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
	return []string{"-az", "-e", fmt.Sprintf("ssh -o StrictHostKeyChecking=yes -o UserKnownHostsFile=%s -i %s", sshFp, sshKey)}
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

	if _, err := os.Stat(dest); err == nil {
		// TODO: add prompt and cmd-flag for force
		return fmt.Errorf("directory already exists: %s", dest)
	}

	return a.runRestore(dest, reply.RsyncUrl+"/", reply.SshFingerprint)
}

func (a *Agent) runRestore(local, remote, sshFp string) error {
	fpFile, err := makeKnownHosts(sshFp)
	if err != nil {
		return err
	}
	//defer os.Remove(fpFile)

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
