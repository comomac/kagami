package client

import (
	"fmt"
	"net/rpc"
	"time"

	"github.com/comomac/kagami/core"
)

// Connect to Server RPC
func Connect(serverIP string) error {

	client, err := rpc.Dial("tcp", serverIP+":"+core.RPCPort)
	if err != nil {
		return err
	}

	// in := bufio.NewReader(os.Stdin)
	for {
		// line, _, err := in.ReadLine()
		// if err != nil {
		// 	return err
		// }
		// var reply int
		// err = client.Call("Listener.GetLine", line, &reply)
		// if err != nil {
		// 	return err
		// }
		// fmt.Println("reply", reply)

		// line, _, err := in.ReadLine()
		// if err != nil {
		// 	return err
		// }
		// n, err := strconv.Atoi(string(line))
		// if err != nil {
		// 	return err
		// }

		var zipImg core.ZipImage
		err = client.Call("Listener.GetZipImage", 0, &zipImg)
		if err != nil {
			return err
		}

		if zipImg.Inode == 0 {
			time.Sleep(time.Second)
			continue
		}

		fmt.Println("zipImg", zipImg.DataSize)

		var reply int
		pHash, w, h, err := core.ProcessImage(zipImg.Data)
		if err != nil {
			zipImg.Error = true
		} else {
			zipImg.Parsed = true
			zipImg.PHash = pHash
			zipImg.Width = w
			zipImg.Height = h
		}
		err = client.Call("Listener.SetZipImage", zipImg, &reply)
		if err != nil {
			return err
		}
	}

	return nil
}
