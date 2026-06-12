///...
package main
import "github.com/motangpuar/o2-ims-worker/internal/config"
import "github.com/motangpuar/o2-ims-worker/internal/tftp"
import "github.com/motangpuar/o2-ims-worker/internal/dhcp"

import (
	"fmt"
)

func main()  {
	//tConfig, dConfig := config.Gather()
	cfg := config.Gather()

	tftpCfgPtr := cfg.TFTP
	dhcpCfgPtr := cfg.DHCP
//----------------------------------------------
	fmt.Println("[DHCP].........")
	fmt.Println(dhcpCfgPtr.BindAddr())
	fmt.Println(dhcpCfgPtr.Enabled())
	fmt.Println(dhcpCfgPtr.Mode())
	fmt.Println(dhcpCfgPtr.BindInterface())
	fmt.Println(dhcpCfgPtr.TFTPIP())
	fmt.Println(dhcpCfgPtr.TFTPPort())
	fmt.Println(dhcpCfgPtr.NextServe())
	fmt.Println(dhcpCfgPtr.BootFilePath())
	fmt.Println()
//----------------------------------------------
	fmt.Println("[TFTP].........")
	fmt.Println(tftpCfgPtr.BindAddr())
	fmt.Println(tftpCfgPtr.Enabled())
	fmt.Println(tftpCfgPtr.BindAddr())
	fmt.Println(tftpCfgPtr.BindPort())
	fmt.Println(tftpCfgPtr.BlockSize())
	fmt.Println(tftpCfgPtr.RootDir())
	fmt.Println()


	e := tftp.NewEngine(tftpCfgPtr)
	e.Start()
	fmt.Println()
	d := dhcp.NewEngine(dhcpCfgPtr)
	d.Start()

}


