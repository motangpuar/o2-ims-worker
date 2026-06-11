package tftp

import "fmt"

// Reacive pointer that have exactly these methods
type Reader interface {
	BindAddr() string
	Enabled() bool
	BindPort() int
	BlockSize() int
	RootDir() string
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
	fmt.Println("[TFTP Realm]--------------")
	fmt.Println(e.cfg.BindAddr())
	fmt.Println(e.cfg.Enabled())
}

