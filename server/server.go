package server

import (
	"archive/zip"
	"flag"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	_ "github.com/comomac/kagami/core"

	"golang.org/x/image/draw"
)

func listZip(file string) (string, error) {
	ino, err := fileInode(file)
	if err != nil {
		return "", err
	}
	fmt.Printf("listing zip (%d) %s\n", ino, file)

	r, err := zip.OpenReader(file)
	if err != nil {
		return "", err
	}
	defer r.Close()

	lines := []string{}
	for _, f := range r.File {
		if !core.reFileExtJPG.MatchString(f.Name) && !core.reFileExtPNG.MatchString(f.Name) {
			continue
		}

		hsh, w, h, err := unzipImageInfo(f)
		if err != nil {
			return "", err
		}

		line := fmt.Sprintf("%08X %9d %04d %04d %016X %s", f.CRC32, f.UncompressedSize64, w, h, hsh, f.Name)
		fmt.Println(line)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n"), nil
}

// phash, w, h
func unzipImageInfo(f *zip.File) (uint64, int, int, error) {
	if !reFileExtJPG.MatchString(f.Name) && !reFileExtPNG.MatchString(f.Name) {
		return 0, 0, 0, fmt.Errorf("not supported image extension")
	}

	rc, err := f.Open()
	if err != nil {
		return 0, 0, 0, err
	}
	img, _, err := image.Decode(rc)
	if err != nil {
		return 0, 0, 0, err
	}
	img2, err := imageResize(img, draw.BiLinear)
	if err != nil {
		return 0, 0, 0, err
	}
	hsh, err := imagePHash(img2)
	if err != nil {
		return 0, 0, 0, err
	}

	rect := img.Bounds().Max

	return hsh, rect.X, rect.Y, nil
}

func startThread(cpu int, ch <-chan string, wg *sync.WaitGroup) {
Loop:
	for {
		select {
		case file := <-ch:
			if file == chExit {
				break Loop
			}

			// # list zip
			txt, err := listZip(file)
			txt = "# kagami_imgsum_ver: 1\n" + "# file: " + file + "\n" + txt

			ino, err := fileInode(file)
			if err != nil {
				log.Println("Error go listZip", file)
				continue
			}
			inoFile := fmt.Sprintf("store/%d.txt", ino)
			saveText(inoFile, txt)

		}
	}
	wg.Done()
}

func listDir(dir string) error {
	fmt.Println("listing dir", dir)

	// start multi-threading
	cpus := runtime.NumCPU() - 1
	ch := make(chan string, cpus)
	var wg sync.WaitGroup
	wg.Add(cpus)
	for i := 0; i < cpus; i++ {
		go startThread(i, ch, &wg)
	}

	err := filepath.Walk(dir, func(file string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !reFileExtCBZ.MatchString(info.Name()) {
			return nil
		}

		// zip file inode
		ino, err := fileInode(file)
		if err != nil {
			return err
		}
		// inode info file
		inoFile := fmt.Sprintf("store/%d.txt", ino)

		fi := fileInfo(inoFile)
		if fi != nil && !fi.IsDir() && fi.Size() > 100 {
			if fi.ModTime().AddDate(0, 0, 7).After(time.Now()) {
				fmt.Println("prev scanned", file)
				return nil
			}
		}

		ch <- file

		return nil
	})
	if err != nil {
		return err
	}

	// fill with exits
	for i := 0; i < cpus; i++ {
		ch <- chExit
	}

	wg.Wait()

	return nil
}

func saveText(file string, txt string) error {
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(txt)
	if err != nil {
		return err
	}

	return nil
}

func main() {
	dirPtr := flag.String("scanDir", "", "dir to scan")

	flag.Parse()

	if dirPtr == nil || *dirPtr == "" {
		fmt.Println("scanDir must be specified")
		return
	}

	// create store dir for inode data
	err := os.Mkdir("store", 0755)
	if err != nil && !os.IsExist(err) {
		log.Fatal(err)
	}

	dir := *dirPtr
	listDir(dir)
}
