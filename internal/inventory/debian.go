package inventory

import (
	"log"
	"os"
	"text/template"
)

// Base

type BaseConfig struct {
	Locale       string
	Keymap       string
	Timezone     string
	Hostname     string
	Domain       string
	Username     string
	UserFullname string
	PasswordHash string
	MirrorHost   string
	MirrorDir    string
	CallbackHost string
	MACAddress   string
}

// OS Specific

type DebianConfig struct {
	BaseConfig
	Interface     string
	DiskDevice    string
	Tasksel       string
	ExtraPackages string
	Suite         string
}

type CentOSConfig struct {
	BaseConfig
	Interface     string
	DiskDevice    string
	ExtraPackages string
}

type UbuntuConfig struct {
	BaseConfig
	Interface     string
	ExtraPackages []string
}

// Generators

func genDebianSeed(id string, mac string) {
	currentConfig := DebianConfig{
		BaseConfig: BaseConfig{
			Locale:       "en_US.UTF-8",
			Keymap:       "us",
			Timezone:     "Asia/Taipei",
			Hostname:     "debian-node",
			Domain:       "local",
			Username:     "debian",
			UserFullname: "Provision User",
			PasswordHash: "$6$60xXa09xhxItPGEF$XCSpaha5jalSX5ylEnwZYDx4AuD.RqBsN19e8.0Ki41OeDaWqokdppne5Lfrbbqh2vH4uyNrdncfPZkcAfg7C0",
			MirrorHost:   "deb.debian.org",
			MirrorDir:    "/debian",
			CallbackHost: "192.168.99.1:8033",
			MACAddress:   mac,
		},
		Interface:     "auto",
		DiskDevice:    "/dev/sda",
		Tasksel:       "standard",
		ExtraPackages: "curl openssh-server efibootmgr",
		Suite:         "bookworm",
	}

	templateFile := "templates/debian.tmpl"
	dumpFile := "assets/http/debian/preseed-" + id + ".cfg"

	tmpl, err := template.ParseFiles(templateFile)
	if err != nil {
		log.Fatalf("[FAIL] failed to parse debian template: %v", err)
	}

	outFile, err := os.Create(dumpFile)
	if err != nil {
		log.Fatalf("[FAIL] failed to create debian seed file: %v", err)
	}
	defer outFile.Close()

	if err := tmpl.Execute(outFile, currentConfig); err != nil {
		log.Fatalf("[FAIL] failed to execute debian template: %v", err)
	}
}

func genCentOSSeed(id string, mac string) {
	currentConfig := CentOSConfig{
		BaseConfig: BaseConfig{
			Locale:       "en_US.UTF-8",
			Keymap:       "us",
			Timezone:     "Asia/Taipei",
			Hostname:     "centos-node",
			Domain:       "local",
			Username:     "centos",
			UserFullname: "Provision User",
			PasswordHash: "$6$60xXa09xhxItPGEF$XCSpaha5jalSX5ylEnwZYDx4AuD.RqBsN19e8.0Ki41OeDaWqokdppne5Lfrbbqh2vH4uyNrdncfPZkcAfg7C0",
			MirrorHost:   "",
			MirrorDir:    "",
			CallbackHost: "192.168.99.1:8033",
			MACAddress:   mac,
		},
		Interface:     "eth0",
		DiskDevice:    "/dev/sda",
		ExtraPackages: "wget openssh-server efibootmgr",
	}

	templateFile := "templates/centos.tmpl"
	dumpFile := "assets/http/centos/kickstart-" + id + ".cfg"

	tmpl, err := template.ParseFiles(templateFile)
	if err != nil {
		log.Fatalf("[FAIL] failed to parse centos template: %v", err)
	}

	outFile, err := os.Create(dumpFile)
	if err != nil {
		log.Fatalf("[FAIL] failed to create centos seed file: %v", err)
	}
	defer outFile.Close()

	if err := tmpl.Execute(outFile, currentConfig); err != nil {
		log.Fatalf("[FAIL] failed to execute centos template: %v", err)
	}
}

func genUbuntuSeed(id string, mac string) {
	currentConfig := UbuntuConfig{
		BaseConfig: BaseConfig{
			Locale:       "en_US.UTF-8",
			Keymap:       "us",
			Timezone:     "Asia/Taipei",
			Hostname:     "ubuntu-node",
			Domain:       "local",
			Username:     "ubuntu",
			UserFullname: "Provision User",
			PasswordHash: "$6$60xXa09xhxItPGEF$XCSpaha5jalSX5ylEnwZYDx4AuD.RqBsN19e8.0Ki41OeDaWqokdppne5Lfrbbqh2vH4uyNrdncfPZkcAfg7C0",
			MirrorHost:   "",
			MirrorDir:    "",
			CallbackHost: "192.168.99.1:8033",
			MACAddress:   mac,
		},
		Interface:     "ens3",
		ExtraPackages: []string{"wget", "openssh-server", "efibootmgr"},
	}

	templateFile := "templates/ubuntu.tmpl"
	dumpFile := "assets/http/ubuntu/user-data-" + id

	tmpl, err := template.ParseFiles(templateFile)
	if err != nil {
		log.Fatalf("[FAIL] failed to parse ubuntu template: %v", err)
	}

	outFile, err := os.Create(dumpFile)
	if err != nil {
		log.Fatalf("[FAIL] failed to create ubuntu user-data file: %v", err)
	}
	defer outFile.Close()

	if err := tmpl.Execute(outFile, currentConfig); err != nil {
		log.Fatalf("[FAIL] failed to execute ubuntu template: %v", err)
	}

	metaFile := "assets/http/ubuntu/meta-data-" + id
	if err := os.WriteFile(metaFile, []byte(""), 0644); err != nil {
		log.Fatalf("[FAIL] failed to create ubuntu meta-data: %v", err)
	}
}
