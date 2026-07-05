package inventory

import "log"
import "text/template"
import "os"

type debianConfig struct {
	Locale        string
	Keymap        string
	Interface     string
	Hostname      string
	Domain        string
	MirrorHost    string
	MirrorDir     string
	Timezone      string
	UserFullname  string
	Username      string
	PasswordHash  string
	Tasksel       string
	ExtraPackages string
	CallbackHost  string
	MACAddress    string
	DiskDevice    string
}

func genDebianSeed(id string, mac string){
	// Populate Valuesk
	currentConfig := debianConfig{
		Locale:        "en_US.UTF-8",
		Keymap:        "us",
		Interface:     "auto",
		Hostname:      "debian-node",
		Domain:        "local",
		MirrorHost:    "deb.debian.org",
		MirrorDir:     "/debian",
		Timezone:      "Asia/Taipei",
		UserFullname:  "Provision User",
		Username:      "debian",
		PasswordHash:  "$6$60xXa09xhxItPGEF$XCSpaha5jalSX5ylEnwZYDx4AuD.RqBsN19e8.0Ki41OeDaWqokdppne5Lfrbbqh2vH4uyNrdncfPZkcAfg7C0",
		Tasksel:       "standard",
		ExtraPackages: "curl openssh-server efibootmgr",
		CallbackHost:  "192.168.99.1:8033/",
		DiskDevice: "/dev/sda",
		MACAddress:    mac,
	}

	// Load Tempalte
	templateFile := "templates/debian.tmpl"
	dumpFile := "assets/http/debian/preseed-"+id+".cfg"
	tmpl, err := template.ParseFiles(templateFile)
	if err != nil {
		log.Fatalf("Failed to parse template %v", err)
	}

	outFile, err := os.Create(dumpFile)
	if err != nil {
		panic(err)
	}

	err = tmpl.Execute(outFile, currentConfig)
	if err != nil {
		panic(err)
	}

	// Parse Template
	// Generate Output
	// assets/debian/preeseed.cfg
}


