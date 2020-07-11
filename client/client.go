package client

import (
	"fmt"
	"net/rpc"
	"runtime"
	"sync"
	"time"

	"github.com/comomac/kagami/core"
)

var (
	sleepTime = time.Millisecond * 300
)

// Connect to Server RPC
func Connect(serverIP string) error {

	client, err := rpc.Dial("tcp", serverIP+":"+core.RPCPort)
	if err != nil {
		return err
	}

	// start multi-threading
	cpus := runtime.NumCPU()
	var wg sync.WaitGroup
	wg.Add(cpus)
	for i := 0; i < cpus; i++ {
		go startThread(i, client, &wg)
	}

	wg.Wait()

	fmt.Println("done")

	return nil
}

func startThread(cpu int, client *rpc.Client, wg *sync.WaitGroup) error {
	fmt.Println("starting thread", cpu)
	var err error

	for {
		var zipImg core.ZipImage
		err = client.Call("Listener.GetZipImage", 0, &zipImg)
		if err != nil {
			return err
		}
		// no data
		if zipImg.Inode == 0 {
			time.Sleep(sleepTime)
			continue
		}
		if zipImg.Inode == -1 {
			fmt.Println("no jobs")
			break
		}

		fmt.Printf("zipImg %d %9d %s\n", cpu, zipImg.DataSize, zipImg.Name)

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

	wg.Done()

	fmt.Println("finishing thread", cpu)

	return nil
}
