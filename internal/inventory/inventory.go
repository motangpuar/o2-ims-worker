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
	Stage2 string
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
type MachineObject struct {
	IP string
	Cluster string
	Template string
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
	Machines map[string]*MachineObject
	InstallTime time.Time
}

type ptrMachines struct {
	Machines map[string]*MachineObject
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

//func Generate(m, t, c,  ansibleTemplate string) {
func (mobj *MachineObject) Generate(ip, m, t, c,  ansibleTemplate string) *MachineObject {
	log.Printf("[Inventory Realm]--------------------")
	log.Printf("[Inventory] Procsesing for %s", m)

	var osDetails any
	var targetMachine MachineObject
	macAsID := strings.ReplaceAll(m, ":", "-")

	targetMachine = MachineObject{
			Cluster: c,
			Template: ansibleTemplate,
			ID: macAsID,
			Installed: false,
			IP: ip,
	}

	switch t {
	case "centos":
		osDetails = CentOSSpecific{
			Initrd: "/images/centos/initrd.img",
			InstallKickStartURL: "http://192.168.99.1:8033/centos/"+macAsID+"/install.ks",
			InstallRepoURL: "http://192.168.99.1:8033/centos/mirror/",
			Stage2: "http://192.168.99.1:8033/centos/mirror",
		}
		targetMachine.OSName="Centos Stream 10"
		targetMachine.OSType=t
		targetMachine.Kernel="/images/centos/vmlinuz"
		targetMachine.OSData=osDetails
		genCentOSSeed(macAsID, m)
	case "ubuntu":
		osDetails = UbuntuSpecific{
			Initrd: "/images/ubuntu/initrd",
			//ISOUrl: "http://192.168.99.1:8033/ubuntu/iso/ubuntu-26.04-desktop-amd64.iso",
			ISOUrl: "http://192.168.99.1:8033/ubuntu/iso/ubuntu-26.04-live-server-amd64.iso",
			CloudConfigURL: "/dev/null",
			DS: "http://192.168.99.1:8033/ubuntu/"+macAsID+"/",
			RootPath: "/dev/ram0",
		}
		targetMachine.OSName="Ubuntu 20.04"
		targetMachine.OSType=t
		targetMachine.Kernel="/images/ubuntu/linux"
		targetMachine.OSData=osDetails
		genUbuntuSeed(macAsID, m)
	case "debian":
		osDetails = DebianSpecific{
			Initrd: "/images/debian/initrd.gz",
			PreeSeedURL: "http://192.168.99.1:8033/debian/preseed-"+macAsID+".cfg",
		}
		targetMachine.OSName="Debian 12"
		targetMachine.OSType=t
		targetMachine.Kernel="/images/debian/linux"
		targetMachine.OSData=osDetails

		// Generate Debian PreSeed
		genDebianSeed(macAsID, m)
	}

	//Generate Grub Entry
	genPXEEntry("bios", m, &targetMachine) 
	genPXEEntry("efi", m, &targetMachine) 

	
	// Append current client as template struct
	machines := activeMachines.Machines
	machines[m] = &targetMachine

	return &targetMachine
}

func genPXEEntry(mode string, m string, targetMachine *MachineObject){

	var dumpFile string
	var templateFile string


	switch mode {
	case "bios":
		templateFile = "templates/main.tmpl"
		dumpFile = "assets/tftp/bios/pxelinux.cfg/01-"+strings.ReplaceAll(m, ":", "-")
	case "efi":
		templateFile = "templates/grub.tmpl"
		dumpFile = "assets/tftp/efi/grub/x86_64-efi/grub.cfg-01-"+strings.ReplaceAll(m, ":", "-")
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
	machines := make(map[string]*MachineObject)
	activeMachines = &ptrMachines{
		Machines: machines,
	}
	activeInstalledMachines = &installedMachines{
		Machines: make(map[string]*MachineObject),
	}
}

func GetTemplate(m string) string {
	target := activeMachines.Machines[m]
	return target.Template
}

func (m *MachineObject) GetTemplate() string {
	return "Hello"
}

