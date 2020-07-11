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
type Listener struct {
	Queue *core.Queue
}

// GetLine test code for RPC
func (l *Listener) GetLine(line []byte, ack *int) error {
	fmt.Println(string(line))
	*ack = 123
	return nil
}

// GetZipImage get the next ZipImage data for RPC
func (l *Listener) GetZipImage(n int, ack *core.ZipImage) error {
	zi := l.Queue.GetNext()
	if zi != nil {
		*ack = *zi
	}
	return nil
}

// SetZipImage set the ZipImage data for RPC
func (l *Listener) SetZipImage(zImg core.ZipImage, ack *int) error {
	fmt.Printf("set! %d %X\n", zImg.Nth, zImg.PHash)
	l.Queue.Set(zImg.Nth, &zImg)
	*ack = 1
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

	q := core.Queue{}
	go core.ListDirByQueue(dir, &q, true)

	listen := listenIP + ":" + core.RPCPort
	fmt.Println("listening", listen)
	addy, err := net.ResolveTCPAddr("tcp", listen)
	if err != nil {
		return err
	}

	inbound, err := net.ListenTCP("tcp", addy)
	if err != nil {
		return err
	}

	listener := new(Listener)
	listener.Queue = &q
	rpc.Register(listener)
	rpc.Accept(inbound)

	return nil
}
