package tftp

import "log"
import "os"
import "github.com/pin/tftp/v3"
import "io"
import "time"
import "strconv"

// Reacive pointer that have exactly these methods
type Reader interface {
	BindAddr() string 
	Enabled() bool
	BindPort() int
	BlockSize() int
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

// Function for filehandler
// TFTP Library start by accepting
// two arguments (readHandler, writeHandler)
func readHandler(filename string, rf io.ReaderFrom) error {
	// Apparently filename is 
	// Told by the user on request
	// via UDP
	// Need to enforce Open Path
	// to limit where this fucker
	// Can get data from


	if filename[0] == '/' {
		filename = filename[1:]
	}

	// Should be ./assets/tftp/{bios,efi}/filename
	file, err := os.Open("./assets/tftp/"+filename)

	if err != nil {
		log.Println("Failed to Open File:", filename)
		return err
	}
	log.Println("[TFTP] Accessing File:", filename)

	// Deffer
	defer file.Close()

	_, err = rf.ReadFrom(file)
	return err
}

// Method of Engine to Start TFTP
// Service
func (e *Engine) Start() {
	log.Println("[TFTP Realm]--------------<*>")
	log.Println(e.cfg.BindAddr())
	log.Println(e.cfg.BindPort())

	tftpAddr := e.cfg.BindAddr()
	tftpPort := e.cfg.BindPort()

	// Start TFTP Hook
	s := tftp.NewServer(readHandler, nil)
	s.SetTimeout(5 * time.Second)

	err := s.ListenAndServe(tftpAddr+":"+strconv.Itoa(tftpPort))
	if err != nil {
		log.Fatalf("TFTP Server failed %v", err)
	}
}

