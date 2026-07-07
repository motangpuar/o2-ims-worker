package inventory

import "log"
import "fmt"
import "text/template"
import "os"
import "strings"
import "time"

type CentOSSpecific struct {
	Initrd string
	IP string
	InstallKickStartURL string
	InstallRepoURL string
}

type RHELSpecific struct {
}

type DebianSpecific struct {
	Initrd string
	PreeSeedURL string
}

type CoreOSSpecifc struct {
	Initrd string
	RootFSURL string
	InstallDev string
	IgnitionURL string }

type UbuntuSpecific struct {
	Initrd string
	IP string
	ISOUrl string
	CloudConfigURL string
	DS string
	RootPath string
}

// Main Struc
type MachineConfig struct {
	ID string
	OSName string
	OSType string
	Kernel string
	OSData any
	Installed bool
}

type installedMachines struct {
	//mac string
	//osType string
	Machines map[string]*MachineConfig
	InstallTime time.Time
}

type ptrMachines struct {
	Machines map[string]*MachineConfig
}

var activeMachines *ptrMachines
var activeInstalledMachines *installedMachines

func FetchMachines() *ptrMachines{
	return activeMachines
}

func FetchInstalledMachines() *installedMachines{
	return activeInstalledMachines
}

func LogInstalledMachine(m, ot string, installedAt time.Time) error {
	targetMachine, exist := activeMachines.Machines[m]
	if exist != true  {
		log.Printf("[Inventory] Invalid MAC: %v", m)
		return fmt.Errorf("Inventory not Exist: %v", m)
	} else {
		activeInstalledMachines.Machines[m] = targetMachine
		activeInstalledMachines.InstallTime = time.Now()
		targetMachine.Installed = true
		//Generate Grub Entry
		genPXEEntry("bios", m, targetMachine) 
		genPXEEntry("efi", m, targetMachine) 

		log.Printf("[Inventory] Foudn Machine, %v", targetMachine)
		log.Printf("[Inventory] Installed object is %v", activeInstalledMachines)
		return nil
	}
}

func Generate(m string, t string) {
	log.Printf("[Inventory Realm]--------------------")
	log.Printf("[Inventory] Procsesing for %s", m)

	var osDetails any
	var targetMachine MachineConfig
	macAsID := strings.ReplaceAll(m, ":", "-")

	targetMachine = MachineConfig{
			ID: macAsID,
			Installed: false,
	}

	switch t {
	case "centos":
		osDetails = CentOSSpecific{
			Initrd: "stream10/initrd.img",
			IP: "dhcp",
			InstallKickStartURL: "http://192.168.99.1:8033/centos10.ks",
			InstallRepoURL: "http://192.168.99.1:8033/mirrors/stream10/",
		}
		targetMachine.OSName="Centos Stream 10"
		targetMachine.OSType=t
		targetMachine.Kernel="stream10/vmlinuz"
		targetMachine.OSData=osDetails
	case "ubuntu":
		osDetails = UbuntuSpecific{
			IP: "dhcp",
			Initrd: "ubuntu/initrd",
			//ISOUrl: "http://192.168.99.1:8033/ubuntu/iso/ubuntu-26.04-desktop-amd64.iso",
			ISOUrl: "http://192.168.99.1:8033/ubuntu/iso/ubuntu-26.04-live-server-amd64.iso",
			CloudConfigURL: "/dev/null",
			DS: "http://192.168.99.1:8033/ubuntu/autoinstall/",
			RootPath: "/dev/ram0",
		}
		targetMachine.OSName="Ubuntu 20.04"
		targetMachine.OSType=t
		targetMachine.Kernel="ubuntu/linux"
		targetMachine.OSData=osDetails
	case "debian":
		osDetails = DebianSpecific{
			Initrd: "debian/initrd.gz",
			PreeSeedURL: "http://192.168.99.1:8033/debian/preseed-"+macAsID+".cfg",
		}
		targetMachine.OSName="Debian 12"
		targetMachine.OSType=t
		targetMachine.Kernel="debian/linux"
		targetMachine.OSData=osDetails
		genDebianSeed(macAsID, m)
	}

	//Generate Grub Entry
	genPXEEntry("bios", m, &targetMachine) 
	genPXEEntry("efi", m, &targetMachine) 

	
	// Append current client as template struct
	machines := activeMachines.Machines
	machines[m] = &targetMachine
}

func genPXEEntry(mode string, m string, targetMachine *MachineConfig){

	var dumpFile string
	var templateFile string


	switch mode {
	case "bios":
		templateFile = "templates/main.tmpl"
		dumpFile = "assets/generic/pxelinux.cfg/01-"+strings.ReplaceAll(m, ":", "-")
	case "efi":
		templateFile = "templates/grub.tmpl"
		dumpFile = "assets/generic/grub.cfg-01-"+strings.ReplaceAll(m, ":", "-")
	}
	log.Printf("Value: mode %s for state %v", mode, targetMachine.Installed)
	tmpl, err := template.ParseFiles(templateFile)
	if err != nil {
		log.Fatalf("Failed to parse template file %v", err)
	}
	
	outFile, err := os.Create(dumpFile)

	if err != nil {
		panic(err)
	}

	defer outFile.Close()

	err = tmpl.Execute(outFile, targetMachine)
	if err != nil {
		panic(err)
	}
}

func Init() {
	machines := make(map[string]*MachineConfig)

	activeMachines = &ptrMachines{
		Machines: machines,
	}

	activeInstalledMachines = &installedMachines{
		Machines: make(map[string]*MachineConfig),
	}

}
