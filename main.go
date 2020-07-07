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
	modeServer := flag.String("server", "localhost", "interface ip to listen from. require --scanDir")
	modeClient := flag.String("client", "localhost", "server ip to connect to")

	dirPtr := flag.String("scanDir", ".", "dir to scan")

	flag.Parse()

	// local mode
	if *modeServer == "" && *modeClient == "" {

		if dirPtr == nil || *dirPtr == "" {
			fmt.Println("scanDir must be specified")
			return
		}

		// create store dir for inode data
		err := os.Mkdir("store", 0755)
		if err != nil && !os.IsExist(err) {
			log.Fatal(err)
		}

		core.ListDir(*dirPtr)

	} else if *modeServer != "" {

		if dirPtr == nil || *dirPtr == "" {
			fmt.Println("scanDir must be specified")
			return
		}

		err := server.Serve(*modeServer, *dirPtr)
		if err != nil {
			log.Fatal(err)
		}

	} else if *modeClient != "" {

		err := client.Connect(*modeClient)
		if err != nil {
			log.Fatal(err)
		}
	}
}
