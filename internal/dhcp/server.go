package dhcp

import "log"
import "net"
import "github.com/insomniacslk/dhcp/dhcpv4"
import "github.com/insomniacslk/dhcp/dhcpv4/server4"

// Reacive pointer that have exactly these methods
type Reader interface {
	BindAddr() string
	BindPort() int
	Enabled() bool
	Mode() string
	BindInterface() string
	TFTPIP() string
	TFTPPort() int
	NextServe() string
	BootFilePath() string
}

// Learn more about this shit
type Engine struct {
	cfg Reader
}

// Process the ability of the pointer
func NewEngine(r Reader) *Engine {
	return &Engine{
		cfg: r,
	}
}

func handler(conn net.PacketConn, peer net.Addr, m *dhcpv4.DHCPv4) {
	log.Println(m.Summary())
}

func (e *Engine) Start() {
	log.Println("[DHCP Realm]--------------<*>")

	bindAddr := e.cfg.BindAddr()
	bindPort := e.cfg.BindPort()

	log.Println(bindAddr)
	log.Println(bindPort)
	log.Println(e.cfg.TFTPIP())

	lAddr := &net.UDPAddr{
		IP: net.ParseIP(bindAddr),
		Port: bindPort,
	}

	log.Println("DHCP Addr: ",lAddr)
	log.Println("DHCP Port: ",bindPort)

	server, err := server4.NewServer("",lAddr, handler)
	if err != nil {
		log.Fatal("Failed to create DHCP server: %v", err)
	}

	if err := server.Serve(); err != nil {
		log.Fatal("DHCP server error: %v", err)
	}
}

