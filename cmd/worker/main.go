///...
package main
import "github.com/motangpuar/o2-ims-worker/internal/config"
import "github.com/motangpuar/o2-ims-worker/internal/tftp"
import "github.com/motangpuar/o2-ims-worker/internal/dhcp"

import (
	"fmt"
	"log"
	"flag"
	"os"
	"os/signal"
	"syscall"
)

func main()  {

	disableTFTP := flag.Bool("no-tftp",false,"Disable TFTP")
	disableDHCP := flag.Bool("no-dhcp",false,"Disable DHCP")
	flag.Parse()
	//tConfig, dConfig := config.Gather()
	cfg := config.Gather()

	tftpCfgPtr := cfg.TFTP
	dhcpCfgPtr := cfg.DHCP
//----------------------------------------------
	log.Println("[DHCP].........")
	log.Println(dhcpCfgPtr.BindAddr())
	log.Println(dhcpCfgPtr.Enabled())
	log.Println(dhcpCfgPtr.Mode())
	log.Println(dhcpCfgPtr.BindInterface())
	log.Println(dhcpCfgPtr.TFTPIP())
	log.Println(dhcpCfgPtr.TFTPPort())
	log.Println(dhcpCfgPtr.NextServe())
	log.Println(dhcpCfgPtr.BootFilePath())
	log.Println()
//----------------------------------------------
	log.Println("[TFTP].........")
	log.Println(tftpCfgPtr.BindAddr())
	log.Println(tftpCfgPtr.Enabled())
	log.Println(tftpCfgPtr.BindAddr())
	log.Println(tftpCfgPtr.BindPort())
	log.Println(tftpCfgPtr.BlockSize())
	log.Println(tftpCfgPtr.RootDir())
	log.Println()

	//log.Println(fd.Clients.MACAddress())
	//log.Println(fd.Clients.OfferIP())
	//log.Println(fd.Clients.BootFileUrl())

	if *disableTFTP != true {
		e := tftp.NewEngine(tftpCfgPtr)
		go e.Start()
	}

	fmt.Println()
	if *disableDHCP != true {
		d := dhcp.NewEngine(dhcpCfgPtr)
		go d.Start()
	}

	// Wait for it to stop
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

}


