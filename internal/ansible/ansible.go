package ansible_worker

import (
	"context"
	_ "embed"
	"log"
	"os"
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

	return f.Name(), func() { os.Remove(f.Name()) }
	//return f.Name(), func() { log.Printf("Write Temp ...")}

}


func Populate() {
	// Debian As Target
	targetIP := "192.168.99.202"
	macAddress := "52:54:00:53:c6:2c"

	// Dummy SSH Key
	sshKey, _ := os.ReadFile("assets/keys/test_provisioner")
	log.Printf("[ANSIBLE] Populate ansible ...")

	machine := template{
		machineType: "k3s",
	}
	log.Printf("[ANSIBLE] Ansible Template %s", machine.machineType)

	playbookFile, cleanupPB := writeTemp("playbook-*-.yaml", playbookYAML, 0644)
	defer cleanupPB()

	keyFile, cleanupKF := writeTemp("id_ed25519-*", sshKey, 0600)

	defer cleanupKF()

	opts := &playbook.AnsiblePlaybookOptions{
		Inventory: targetIP+",",
		User: "debian",
		PrivateKey: keyFile,
	}

	opts.AddExtraVar("mac_address", macAddress)

	cwd, err := os.Getwd()
	if err != nil {
		log.Printf("[ANSIBLE] Failed to find path: %v", err)
		return
	}

	kubeconfigDest := filepath.Join(cwd, "assets", "ansible", "kubeconfig-target.yaml")
	log.Printf("[ANSIBLE] Target path: %s", kubeconfigDest)

	opts.AddExtraVar("kubeconfig_dest", kubeconfigDest)
	opts.AddExtraVar("ansible_become_pass", "password")

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

func Gather() {
	log.Printf("[ANSIBLE] Running ansible agent...")
}

