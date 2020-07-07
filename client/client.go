package client

import (
	"bufio"
	"fmt"
	"net/rpc"
	"os"

	"github.com/comomac/kagami/core"
)

// Connect to Server RPC
func Connect(serverIP string) error {

	client, err := rpc.Dial("tcp", serverIP+":"+core.RPCPort)
	if err != nil {
		return err
	}

	in := bufio.NewReader(os.Stdin)
	for {
		line, _, err := in.ReadLine()
		if err != nil {
			return err
		}
		var reply int
		err = client.Call("Listener.GetLine", line, &reply)
		if err != nil {
			return err
		}
		fmt.Println("reply", reply)
	}

	return nil
}
