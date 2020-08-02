package core

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/image/draw"
)

var (
	reFileExtCBZ *regexp.Regexp = regexp.MustCompile("(?i)\\.cbz$")
	reFileExtJPG *regexp.Regexp = regexp.MustCompile("(?i)\\.jp(e|)g$")
	reFileExtPNG *regexp.Regexp = regexp.MustCompile("(?i)\\.png$")
	sleepTime    time.Duration  = time.Millisecond * 300
)

// constants
const (
	chExit  = ":exit:"
	RPCPort = "4122"
)

func fileInode(file string) (uint64, error) {
	fileinfo, _ := os.Stat(file)
	stat, ok := fileinfo.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, fmt.Errorf("Not a syscall.Stat_t")
	}
	return stat.Ino, nil
}

// fileInfo only check file info, not dir
func fileInfo(file string) os.FileInfo {
	info, err := os.Stat(file)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return nil
	}
	if info.IsDir() {
		return nil
	}

	return info
}

// ListDir recursively list directory looking for cbz and queue jobs
func ListDir(dir string) error {
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

// Queue for images in a zip file
type Queue struct {
	name string            // zip file name
	ino  uint64            // zip file inode
	zs   map[int]*ZipImage // in-memory image data
	ds   map[int]bool      // 1:1 map to zs, marking ZipImage as done (regardless of success or failure)
	cur  int               // cursor, last image nth
	len  int               // queue length
	mux  sync.Mutex        // read/write control flag
	fin  bool              // finish (all zips) flag
}

// GetNext next ZipImage
func (q *Queue) GetNext() *ZipImage {
	q.mux.Lock()
	if q.fin {
		q.mux.Unlock()
		return &ZipImage{
			Inode: -1,
		}
	}
	zi := q.zs[q.cur]
	q.cur++
	q.mux.Unlock()
	return zi
}

// Get nth ZipImage
func (q *Queue) Get(n int) *ZipImage {
	q.mux.Lock()
	zi := q.zs[n]
	q.mux.Unlock()
	return zi
}

// Set nth ZipImage
func (q *Queue) Set(n int, in *ZipImage) error {
	zi := q.Get(n)
	if zi == nil {
		return fmt.Errorf("nth ZipImage not exist (%d)", n)
	}

	q.mux.Lock()
	zipImg := q.zs[n]

	if in.Error == true {
		zipImg.Error = true
	} else {
		zipImg.Parsed = true
		zipImg.PHash = in.PHash
		zipImg.Width = in.Width
		zipImg.Height = in.Height
	}
	q.ds[n] = true
	q.mux.Unlock()

	return nil
}

// ListDirByQueue recursively list directory looking for cbz and queue jobs by images
func ListDirByQueue(dir string, q *Queue, serverMode bool) error {
	fmt.Println("listing dir by images", dir)

	if !serverMode {
		// start multi-threading
		cpus := runtime.NumCPU()
		for i := 0; i < cpus; i++ {
			go startThreadByQueue(i, q)
		}
	}

	err := filepath.Walk(dir, func(file string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasPrefix(info.Name(), ".") {
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

		// -- producer --
		r, err := zip.OpenReader(file)
		if err != nil {
			return err
		}
		defer r.Close()

		q.mux.Lock()
		q.ino = ino
		q.name = file
		q.cur = 0
		q.zs = map[int]*ZipImage{}
		q.ds = map[int]bool{}
		q.mux.Unlock()

		rtotal := 0
		for _, f := range r.File {
			if !reFileExtJPG.MatchString(f.Name) && !reFileExtPNG.MatchString(f.Name) {
				continue
			}

			fp, err := f.Open()
			if err != nil {
				return err
			}
			fdat, err := ioutil.ReadAll(fp)
			if err != nil {
				return err
			}

			// add queue
			q.mux.Lock()
			q.zs[rtotal] = &ZipImage{
				MTime:    info.ModTime(),
				Name:     f.Name,
				Inode:    int64(ino),
				Nth:      rtotal,
				CRC32:    f.CRC32,
				Data:     fdat,
				DataSize: f.UncompressedSize64,
			}
			q.ds[rtotal] = false
			q.mux.Unlock()

			rtotal++
		}
		q.mux.Lock()
		q.len = rtotal
		q.mux.Unlock()
		// -- /producer --

		// blocking
		// check if zip file is processed
	FCheck:
		for {
			i := 0

			q.mux.Lock()
			len := q.len
			for _, b := range q.ds {
				if b {
					i++
				}
			}
			q.mux.Unlock()

			if i >= len-1 {
				break FCheck
			}
			time.Sleep(sleepTime)
		}

		// print zip images result
		q.mux.Lock()
		zs := []*ZipImage{}
		for _, zz := range q.zs {
			zs = append(zs, zz)
		}
		// sort by filename
		sort.Slice(zs, func(i, j int) bool {
			return zs[i].Name < zs[j].Name
		})
		// save phash record
		txt := "# kagami_imgsum_ver: 1\n" + "# file: " + file + "\n"
		for _, zz := range zs {
			line := fmt.Sprintf("%08X %9d %04d %04d %016X %s", zz.CRC32, zz.DataSize, zz.Width, zz.Height, zz.PHash, zz.Name)
			fmt.Println(line)
			txt += line + "\n"
		}
		saveText(inoFile, txt)
		q.mux.Unlock()

		return nil
	})
	if err != nil {
		return err
	}

	q.mux.Lock()
	q.fin = true
	q.mux.Unlock()

	if serverMode {
		// kill and exit
		log.Fatal("DONE")
	}

	fmt.Println("DONE")
	return nil
}

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
		if !reFileExtJPG.MatchString(f.Name) && !reFileExtPNG.MatchString(f.Name) {
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

// unzipImageInfo create phash from zip.File.
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

// ProcessImage produce image phash, width, height
func ProcessImage(dat []byte) (uint64, int, int, error) {
	r := bytes.NewReader(dat)

	img, _, err := image.Decode(r)
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

// scan base on zip file
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

// ZipImage individual image file detail from zip file
type ZipImage struct {
	MTime    time.Time // zip file modified time
	Inode    int64     // zip file inode, -1 means stop for rpc
	Nth      int       // image file order in zip
	CRC32    uint32    // image data crc32
	Name     string    // image file path+name
	Data     []byte    // image data
	DataSize uint64    // image data size
	Parsed   bool      // is image phashed
	Error    bool      // is error
	PHash    uint64    // image phash
	Width    int       // image width
	Height   int       // image height
}

// scan base on image data
func startThreadByQueue(cpu int, q *Queue) {
	fmt.Println("starting thread", cpu)

	for {
		if q.cur >= q.len {
			// todo wait until all others are finished

			if q.fin {
				// everything is done
				break
			}

			time.Sleep(sleepTime)
			continue
		}

		zipImg := q.zs[q.cur]
		if zipImg == nil {
			// not ready yet
			time.Sleep(sleepTime)
			continue
		}

		q.mux.Lock()
		cursor := q.cur
		q.cur++
		q.mux.Unlock()

		pHash, w, h, err := ProcessImage(zipImg.Data)
		q.mux.Lock()
		if err != nil {
			zipImg.Error = true
		} else {
			zipImg.Parsed = true
			zipImg.PHash = pHash
			zipImg.Width = w
			zipImg.Height = h
		}
		q.ds[cursor] = true
		q.mux.Unlock()
	}

	fmt.Println("finished thread", cpu)
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

func imageResize(src image.Image, resizeMethod draw.Interpolator) (image.Image, error) {
	if src == nil {
		return nil, fmt.Errorf("image is nil")
	}

	// dest image rect dimension
	dr := image.Rect(0, 0, 8, 8)

	dst := image.NewRGBA(dr)

	draw.Scaler.Scale(resizeMethod, dst, dr, src, src.Bounds(), draw.Over, nil)

	return dst, nil
}

func calcGray(c color.Color) uint8 {
	r, g, b, _ := c.RGBA()

	// These coefficients (the fractions 0.299, 0.587 and 0.114) are the same
	// as those given by the JFIF specification and used by func RGBToYCbCr in
	// ycbcr.go.
	//
	// Note that 19595 + 38470 + 7471 equals 65536.
	//
	// The 24 is 16 + 8. The 16 is the same as used in RGBToYCbCr. The 8 is
	// because the return value is 8 bit color, not 16 bit color.
	y := (19595*r + 38470*g + 7471*b + 1<<15) >> 24

	// black == 0
	// white == 255

	return uint8(y)
}

func imageOpen() (image.Image, error) {
	// file := "00001.jpeg"
	file := "001.png"

	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, format, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	fmt.Println("img format is", format)

	return img, nil
}

func imageSave(out string, img image.Image) error {
	dst, err := os.Create(out)
	if err != nil {
		return err
	}
	err = png.Encode(dst, img)
	if err != nil {
		return err
	}
	return nil
}

func basicResizeBench(img image.Image) error {
	resizeMethods := []draw.Interpolator{
		draw.NearestNeighbor,
		draw.ApproxBiLinear,
		draw.BiLinear,
		draw.CatmullRom,
	}
	for i, resizeMethod := range resizeMethods {
		tp := time.Now()
		img2, err := imageResize(img, resizeMethod)
		if err != nil {
			return err
		}
		log.Printf("scaling using %d takes %v time",
			i, time.Now().Sub(tp))

		err = imageSave(fmt.Sprintf("out-%d.png", i), img2)
		if err != nil {
			return err
		}
	}
	return nil
}

func imagePHashFile(file string) (uint64, error) {
	f, err := os.Open(file)
	if err != nil {
		return 0, err
	}

	img, format, err := image.Decode(f)
	if err != nil {
		return 0, err
	}
	if format != "png" {
		return 0, fmt.Errorf("image not png")
	}

	return imagePHash(img)
}

func imagePHash(img image.Image) (uint64, error) {
	mx := img.Bounds().Max.X
	my := img.Bounds().Max.Y

	if mx != 8 {
		return 0, fmt.Errorf("image width not 8")
	}
	if my != 8 {
		return 0, fmt.Errorf("image height not 8")
	}

	// calc global average Luminance
	ttl := 0
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			c := img.At(x, y)
			g := calcGray(c)
			ttl += int(g)
		}
	}
	gLum := ttl / 64

	var b uint64
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			c := img.At(x, y)
			g := calcGray(c)
			if int(g) > gLum {
				b = 1 | b<<1
			} else {
				b = 0 | b<<1
			}
		}
	}

	return b, nil
}
