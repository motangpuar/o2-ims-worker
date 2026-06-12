package tftp

import "fmt"
import "log"
import "os"
import "path/filepath"
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

	file, err := os.Open("/tmp/tftp/"+filename)

	if err != nil {
		log.Println("Failed to Open File %s", filename)
	}

	// Deffer
	defer file.Close()

	_, err = rf.ReadFrom(file)
	return err
}

// Method of Engine to Start TFTP
// Service
func (e *Engine) Start() {
	fmt.Println("[TFTP Realm]--------------<*>")
	fmt.Println(e.cfg.BindAddr())
	fmt.Println(e.cfg.BindPort())

	rootDir := e.cfg.RootDir()
	tftpAddr := e.cfg.BindAddr()
	tftpPort := e.cfg.BindPort()

	if _, err := os.Stat(rootDir); os.IsNotExist(err) {
		log.Fatalf("TFTP Directory Error: %s", rootDir)
	}

	// Internal Handler
	filename := "pxelinux.0"

	if filename[0] == '/' {
		filename = filename[1:]
	}

	fullPath := filepath.Join(rootDir, filename)

	fmt.Println(fullPath)

	// Start TFTP Hook
	s := tftp.NewServer(readHandler, nil)
	s.SetTimeout(5 * time.Second)

	err := s.ListenAndServe(tftpAddr+":"+strconv.Itoa(tftpPort))
	if err != nil {
		log.Fatalf("TFTP Server failed %v", err)
	}
}

