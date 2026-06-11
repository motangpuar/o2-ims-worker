package dhcp

import "fmt"

// Reacive pointer that have exactly these methods
type Reader interface {
	BindAddr() string
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

func (e *Engine) Start() {
	fmt.Println("[DHCP Realm]--------------")
	fmt.Println(e.cfg.BindAddr())
	fmt.Println(e.cfg.Enabled())
}

