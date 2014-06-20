package rolly

import (
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
	conn, err := secrpc.SecureDial("tcp", backendAddr, RollbackupCA)
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

func (a *Agent) execTask(task rb.Task, results chan TaskResult) {
	f, err := ioutil.TempFile("", "rbd_known_hosts")
	if err != nil {
		log.Println(err)
		return
	}

	log.Println(f.Name())
	defer os.Remove(f.Name())
	f.WriteString(task.SshFingerprint)

	args := []string{"-aze", fmt.Sprintf("ssh -o StrictHostKeyChecking=yes -o UserKnownHostsFile=%s -i %s", f.Name(), KeyPath)}
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
