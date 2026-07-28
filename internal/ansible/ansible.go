package ansible_worker

import (
	"context"
	_ "embed"
	"log"
	"os"
    "strings"
	"path/filepath"

	"github.com/apenella/go-ansible/v2/pkg/execute"
	"github.com/apenella/go-ansible/v2/pkg/playbook"
)

//go:embed templates/playbook.yaml
var playbookYAML []byte

//go:embed templates/uninstall.yaml
var uninstallPlaybook []byte

type template struct {
	machineType string
}

type AnsibleInfo struct {
	KubeconfigPath string
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

func Populate(targetIP, macAddress, userName string) {

	// Dummy SSH Key
	sshKey, _ := os.ReadFile("assets/keys/test_provisioner")
	log.Printf("[ANSIBLE] Populate ansible ...")

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
		return
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

	exec := execute.NewDefaultExecute(
		execute.WithCmd(cmd),
		execute.WithErrorEnrich(playbook.NewAnsiblePlaybookErrorEnrich()),
	)

	if err := exec.Execute(context.Background()); err != nil {
		log.Printf("[Ansible] Fail to execute ansible: %v", err)
		return
	}

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

