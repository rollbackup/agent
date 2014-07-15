package rolly

import (
	"errors"
	"bytes"
	"fmt"
	"github.com/rollbackup/rb"
	"github.com/rollbackup/secrpc"
	"io/ioutil"
	"log"
	"net/rpc"
	"net/rpc/jsonrpc"
	"os"
	"os/exec"
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

func (a *Agent) GetBackup(backupId string) error {
	args := rb.HostGetBackupParams{Auth: *a.auth, BackupId: backupId}
	var reply rb.HostGetBackupResult
	err := a.backend.Call("Host.GetBackup", args, &reply)
	log.Printf("%+v", reply)
	return err
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

	if reply.Success && len(reply.Tasks) > 0 {
		log.Printf("%+v", reply.Tasks)
		a.execTasks(reply.Tasks)
	}

	return nil
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
	return []string{"-avz", "-e", fmt.Sprintf("ssh -o StrictHostKeyChecking=yes -o UserKnownHostsFile=%s -i %s", sshFp, sshKey)}
}

func (a *Agent) execTask(task rb.Task, results chan TaskResult) {
	fpFile, err := makeKnownHosts(task.SshFingerprint)
	if err != nil {
		log.Println(err)
		return
	}
	defer os.Remove(fpFile)

	args := buildRsyncArgs(fpFile, KeyPath)
	if task.LinkDest != "" {
		args = append(args, fmt.Sprintf("--link-dest=%s", task.LinkDest))
	}
	args = append(args, task.Local, task.Remote)
	log.Println(args)
	cmd := exec.Command("rsync", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	err = cmd.Run()
	res := TaskResult{Task: task, Output: out.String()}
	if err != nil {
		res.Error = err
	}
	results <- res
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

	return a.runRestore(dest, reply.RsyncUrl + "/", reply.SshFingerprint)
}

func (a *Agent) runRestore(local, remote, sshFp string) error {
	fpFile, err := makeKnownHosts(sshFp)
	if err != nil {
		return err
	}
	defer os.Remove(fpFile)

	args := buildRsyncArgs(fpFile, KeyPath)
	args = append(args, remote, local)

	//log.Println(args)
	cmd := exec.Command("rsync", args...)

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

func (a *Agent) execTasks(tasks []rb.Task) error {
	results := make(chan TaskResult)
	for _, t := range tasks {
		log.Printf("run task: %+v", t)
		go a.execTask(t, results)
	}

	for i := 0; i < len(tasks); i++ {
		result := <-results
		// TODO: send error
		if result.Error == nil {
			a.commitBackup(&result.Task)
		}
		log.Printf("%+v", result)
	}

	return nil
}

func (a *Agent) commitBackup(task *rb.Task) error {
	args := rb.HostCommitBackupParams{Auth: *a.auth, FolderId: task.FolderId, BackupId: task.BackupId}
	var reply rb.HostOpResult
	return a.backend.Call("Host.CommitBackup", args, &reply)
}
