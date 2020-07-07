package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {
	modeLocal := flag.String("local", ".", "process locally")
	modeServer := flag.String("server", "localhost", "interface ip to listen from")
	modeClient := flag.String("client", "localhost", "server ip to connect to")

	dirPtr := flag.String("scanDir", ".", "dir to scan")

	flag.Parse()

	if *modeLocal == "local" {

		if dirPtr == nil || *dirPtr == "" {
			fmt.Println("scanDir must be specified")
			return
		}

		// create store dir for inode data
		err := os.Mkdir("store", 0755)
		if err != nil && !os.IsExist(err) {
			log.Fatal(err)
		}

		core.listDir(*dirPtr)

	} else if *modeServer == "server" {

	} else if *modeClient == "client" {

	}
}
