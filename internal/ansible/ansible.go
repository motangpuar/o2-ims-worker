package ansible_worker

import (
	"fmt"
	"bytes"
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"encoding/json"

	"github.com/apenella/go-ansible/v2/pkg/playbook"
	"github.com/apenella/go-ansible/v2/pkg/execute"
	ansiblejson "github.com/apenella/go-ansible/v2/pkg/execute/result/json"
	"github.com/apenella/go-ansible/v2/pkg/execute/stdoutcallback"


	//Internals
	"github.com/motangpuar/o2-ims-worker/internal/kubernetes"
)

type template struct {
	machineType string
}

type AnsibleInfo struct {
	Cluster kubeclient.ClusterInfo
	KubeconfigPath string
}

type AnsibleJSONResults struct {
	Plays []struct {
		Tasks []struct {
			Hosts map[string]struct {
				Msg interface{} `json:"msg"`
			} `json:"hosts"`
		} `json:"tasks"`
	} `json:"plays"`
}

type K3sCreds struct {
	Server string `json:"server"`
	Token string `json:"token"`
}

var PtrAnsibleInfi *AnsibleInfo

func GetInfo() *AnsibleInfo {
	return PtrAnsibleInfi
}

type K3sConfig struct {
	ServerIP          string
	DisableComponents string
	// CPU
	CPUManagerPolicy  string // static or none
	SystemReservedCPU string // e.g. "0-1"
	KubeReservedCPU   string // e.g. "2"
	// Memory
	SystemReservedMem string // e.g. "512Mi"
	KubeReservedMem   string // e.g. "256Mi"
}

func writeTemp(pattern string, data []byte, perm os.FileMode)(string, func()){
	f,err := os.CreateTemp("", pattern)
	if err != nil {
		log.Printf("[ANSIBLE] Error Create Temp file for ansible: %v", err)
		return "",nil
	}

	f.Write(data)
	f.Close()
	os.Chmod(f.Name(), perm)

	log.Printf("[ANSIBLE] Temp File name is %s", f.Name())

	return f.Name(), func() { log.Printf("Not Delete") }
	//return f.Name(), func() { os.Remove(f.Name()) }

}

func extractCreds(res ansiblejson.AnsiblePlaybookJSONResults) (*K3sCreds, error) {
	log.Printf("[EXTRACTION] %T", res)
	for _, play := range res.Plays {
		for _, task := range play.Tasks {
			for _, host := range task.Hosts {
				if host.Msg == nil {
					continue
				}
				s, ok := host.Msg.(string)
				if !ok || s == "" {
					continue
				}
				var creds K3sCreds
				if err := json.Unmarshal([]byte(s), &creds); err != nil {
					continue
				}
				if creds.Server != "" && creds.Token != "" {
					return &creds, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("no valid k3s credentials found")
}


func FetchToken(targetIP, userName, macAddress string) (*K3sCreds, error) {

	sshKey, _ := os.ReadFile("assets/keys/test_provisioner")

	var playbookYAML []byte
	chunk, err := os.ReadFile("templates/ansible/k3s-fetch-token.yaml")
	if err != nil {
		return nil,err
	}
	playbookYAML = chunk

	playbookFile, cleanupPB := writeTemp("playbook-*.yaml", playbookYAML, 0644)
	defer cleanupPB()

	keyFile, cleanupKF := writeTemp("id_ed25519-*", sshKey, 0600)
	defer cleanupKF()
	
	opts := &playbook.AnsiblePlaybookOptions{
		Inventory: targetIP+",",
		User: userName,
		PrivateKey: keyFile,
		SSHExtraArgs:  "-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -tt",
		BecomeMethod: "sudo",
	}

	cwd, err := os.Getwd()
	macAsID := strings.ReplaceAll(macAddress, ":", "-")
    filePattern := "kubeconfig-"+macAsID+".yaml"

	kubeconfigDest := filepath.Join(cwd, "assets", "ansible", filePattern)
	
	PtrAnsibleInfi = &AnsibleInfo{
		KubeconfigPath: kubeconfigDest,
	}

	opts.AddExtraVar("ansible_become_pass", "password")
	opts.AddExtraVar("mac_address", macAddress)
	if userName == "ubuntu" {
		opts.AddExtraVar("ansible_become_exe", "sudo.ws")
	}

	cmd := playbook.NewAnsiblePlaybookCmd(
		playbook.WithPlaybooks(playbookFile),
		playbook.WithPlaybookOptions(opts),
	)

	var buff bytes.Buffer
	exec := stdoutcallback.NewJSONStdoutCallbackExecute(
		execute.NewDefaultExecute(
			execute.WithCmd(cmd),
			execute.WithWrite(&buff),
		),
	)

	if err := exec.Execute(context.Background()); err != nil {
		log.Printf("[Ansible] Fail to execute ansible: %v", err)
		return nil,err
	}


	res, err := ansiblejson.ParseJSONResultsStream(&buff)
	dump,err := extractCreds(*res)
	if err != nil {
		log.Printf("[ANSIBLE] Error: %v", err.Error())
		return nil,err
	}

	return dump, nil
}

func Populate(targetIP, macAddress, userName, templateMode string) (*ansiblejson.AnsiblePlaybookJSONResults,error) {

	// Dummy SSH Key
	sshKey, _ := os.ReadFile("assets/keys/test_provisioner")
	log.Printf("[ANSIBLE] Populate ansible ...")

	var playbookYAML []byte
	switch templateMode { 
	case "k3s-master":
		chunk, err := os.ReadFile("templates/ansible/k3s-master.yaml")
		if err != nil {
			return nil,err
		}
		playbookYAML = chunk
	case "k3s-worker":
		chunk, err := os.ReadFile("templates/ansible/k3s-worker.yaml")
		if err != nil {
			return nil,err
		}
		playbookYAML = chunk
	}

	log.Printf("[ANSIBLE] template %s file size %d", templateMode, len(playbookYAML))

	machine := template{
		machineType: "k3s",
	}
	log.Printf("[ANSIBLE] Ansible Template %s", machine.machineType)

	playbookFile, cleanupPB := writeTemp("playbook-*.yaml", playbookYAML, 0644)
	defer cleanupPB()

	keyFile, cleanupKF := writeTemp("id_ed25519-*", sshKey, 0600)

	defer cleanupKF()

	os.Setenv("ANSIBLE_HOST_KEY_CHECKING", "False")
	opts := &playbook.AnsiblePlaybookOptions{
		Inventory: targetIP+",",
		User: userName,
		PrivateKey: keyFile,
		SSHExtraArgs:  "-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -tt",
		BecomeMethod: "sudo",
	}

	opts.AddExtraVar("mac_address", macAddress)

	cwd, err := os.Getwd()
	if err != nil {
		log.Printf("[ANSIBLE] Failed to find path: %v", err)
		return nil,err
	}

	macAsID := strings.ReplaceAll(macAddress, ":", "-")
    filePattern := "kubeconfig-"+macAsID+".yaml"

	kubeconfigDest := filepath.Join(cwd, "assets", "ansible", filePattern)
	
	PtrAnsibleInfi = &AnsibleInfo{
		KubeconfigPath: kubeconfigDest,
	}
	
	log.Printf("[ANSIBLE] Target path: %s", kubeconfigDest)

	opts.AddExtraVar("ansible_host", targetIP)
	opts.AddExtraVar("kubeconfig_dest", kubeconfigDest)
	opts.AddExtraVar("ansible_become_pass", "password")
	if userName == "ubuntu" {
		opts.AddExtraVar("ansible_become_exe", "sudo.ws")
	}

	if userName == "centos" {
    opts.AddExtraVar("k3s_extra_flags", "--write-kubeconfig-mode 644 --prefer-bundled-bin")
	} else {
	    opts.AddExtraVar("k3s_extra_flags", "--write-kubeconfig-mode 644")
	}

	cmd := playbook.NewAnsiblePlaybookCmd(
		playbook.WithPlaybooks(playbookFile),
		playbook.WithPlaybookOptions(opts),
	)

	var buf bytes.Buffer
	exec := stdoutcallback.NewJSONStdoutCallbackExecute(
		execute.NewDefaultExecute(
			execute.WithCmd(cmd),
			execute.WithWrite(&buf),
			execute.WithErrorEnrich(playbook.NewAnsiblePlaybookErrorEnrich()),
		),
	)

	if err := exec.Execute(context.Background()); err != nil {
		log.Printf("[Ansible] Fail to execute ansible: %v", err)
		return nil,err
	}

	res, err := ansiblejson.ParseJSONResultsStream(&buf)
	return res,nil
}

func InitAnsible() {
	cwd, err := os.Getwd()
	if err != nil {
		log.Printf("[ANSIBLE] Failed to find path: %v", err)
		return
	}
	kubeconfigDest := filepath.Join(cwd, "assets", "ansible", "kubeconfig-target.yaml")

	PtrAnsibleInfi = &AnsibleInfo{
		KubeconfigPath: kubeconfigDest,
	}
}

func Gather() {
	log.Printf("[ANSIBLE] Running ansible agent...")
}

