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
	mode := flag.String("mode", "help", "mode to run. server, client, local, check, rmdup")
	hostIP := flag.String("hostIP", "", "server ip to host from or connect ip (server/client)")

	dirPtr := flag.String("scanDir", ".", "dir to scan")

	flag.Parse()

	switch *mode {
	case "local":
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
	case "server":
		// server mode
		fmt.Println("mode: server")

		if dirPtr == nil || *dirPtr == "" {
			fmt.Println("scanDir must be specified")
			return
		}

		err := server.Serve(*hostIP, *dirPtr)
		if err != nil {
			log.Fatal(err)
		}
	case "client":
		// client mode
		fmt.Println("mode: client")

		err := client.Connect(*hostIP)
		if err != nil {
			log.Fatal(err)
		}

	case "help":
	default:
		printHelp()
	}

}

func printHelp() {
	fmt.Println(`Kagami - detect duplicate image in archive
modes:
  server - holds archive and send images to client to create image sums
  client - receive images and calculate image sums
  local - calculate image sums locally
  check - find archives with duplicate images

parameters:
  scanDir - directory to scan archives
  hostIP - server/client use. server: ip for server to host from. client: server ip to connect to`)
}
