package server

import (
	"fmt"
	_ "image/gif"
	_ "image/jpeg"
	"os"

	"github.com/comomac/kagami/core"
)

func Serve(listenIP, dir string) error {
	if dir == "" {
		return fmt.Errorf("scanDir must be specified")
	}

	// create store dir for inode data
	err := os.Mkdir("store", 0755)
	if err != nil && !os.IsExist(err) {
		return err
	}

	core.ListDir(dir)

	return nil
}
