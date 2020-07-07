package server

import (
	"fmt"
	_ "image/gif"
	_ "image/jpeg"
	"net"
	"net/rpc"
	"os"

	"github.com/comomac/kagami/core"
)

// Listener RPC interface
type Listener int

// GetLine test code for RPC
func (l *Listener) GetLine(line []byte, ack *int) error {
	fmt.Println(string(line))
	*ack = 123
	return nil
}

// Serve initialise RCP service
func Serve(listenIP, dir string) error {
	if listenIP == "" {
		listenIP = "localhost"
	}
	if dir == "" {
		return fmt.Errorf("scanDir must be specified")
	}

	// create store dir for inode data
	err := os.Mkdir("store", 0755)
	if err != nil && !os.IsExist(err) {
		return err
	}

	// core.ListDir(dir)

	addy, err := net.ResolveTCPAddr("tcp", listenIP+":"+core.RPCPort)
	if err != nil {
		return err
	}

	inbound, err := net.ListenTCP("tcp", addy)
	if err != nil {
		return err
	}

	listener := new(Listener)
	rpc.Register(listener)
	rpc.Accept(inbound)

	return nil
}
