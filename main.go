package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/comomac/kagami/client"
	"github.com/comomac/kagami/core"
	"github.com/comomac/kagami/server"
)

func main() {
	modeServer := flag.String("server", "", "interface ip to listen from. require --scanDir")
	modeClient := flag.String("client", "", "server ip to connect to")

	dirPtr := flag.String("scanDir", ".", "dir to scan")

	flag.Parse()

	if *modeServer == "" && *modeClient == "" {
		// local mode
		fmt.Println("mode: local")

		if dirPtr == nil || *dirPtr == "" {
			fmt.Println("scanDir must be specified")
			return
		}

		// create store dir for inode data
		err := os.Mkdir("store", 0755)
		if err != nil && !os.IsExist(err) {
			log.Fatal(err)
		}

		// list by files
		// core.ListDir(*dirPtr)

		// list by images
		q := core.Queue{}
		core.ListDirByQueue(*dirPtr, &q, false)

	} else if *modeServer != "" {
		// server mode
		fmt.Println("mode: server")

		if dirPtr == nil || *dirPtr == "" {
			fmt.Println("scanDir must be specified")
			return
		}

		err := server.Serve(*modeServer, *dirPtr)
		if err != nil {
			log.Fatal(err)
		}

	} else if *modeClient != "" {
		// client mode
		fmt.Println("mode: client")

		err := client.Connect(*modeClient)
		if err != nil {
			log.Fatal(err)
		}
	}
}
