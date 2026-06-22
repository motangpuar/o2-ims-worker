package inventory

import "log"
import "text/template"
import "os"
import "strings"

type CentOSSpecific struct {
	Initrd string
	IP string
	InstallKickStartURL string
	InstallRepoURL string
}

type RHELSpecific struct {
}

type CoreOSSpecifc struct {
	Initrd string
	RootFSURL string
	InstallDev string
	IgnitionURL string
}

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
	OSName string
	OSType string
	Kernel string
	OSData any
}

func Generate(m string, t string) {
	log.Printf("[Inventory Realm]--------------------")
	log.Printf("[Inventory] Procsesing for %s", m)

	var osDetails any
	var targetMachine MachineConfig
	switch t {
	case "centos":
		osDetails = CentOSSpecific{
			Initrd: "stream10/initrd.img",
			IP: "dhcp",
			InstallKickStartURL: "http://192.168.99.1:8033/centos10.ks",
			InstallRepoURL: "http://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/",
		}
		targetMachine = MachineConfig{
			OSName: "Centos Stream 10",
			OSType: t,
			Kernel: "stream10/vmlinuz",
			OSData: osDetails,
		}
	case "ubuntu":
		osDetails = UbuntuSpecific{
			Initrd: "ubuntu/initrd.img",
			IP: "dhcp",
			ISOUrl: "https://aaaaaa/bbbb",
			CloudConfigURL: "https://aaaaaa/bbbb",
			DS: "https://aaaaaa/bbbb/metadata/",
			RootPath: "/dev/ram0",
		}
		targetMachine = MachineConfig{
			OSName: "Ubuntu 20.04",
			OSType: t,
			Kernel: "ubuntu20.04/vmlinuz",
			OSData: osDetails,
		}
	}

	tmpl, err := template.ParseFiles("templates/main.tmpl")
	if err != nil {
		log.Fatalf("Failed to parse template file %v", err)
	}

	dumpFile := "assets/generic/pxelinux.cfg/01-"+strings.ReplaceAll(m, ":", "-")
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

