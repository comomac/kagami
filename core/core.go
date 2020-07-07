package core

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"regexp"
	"syscall"
	"time"

	"golang.org/x/image/draw"
)

var (
	reFileExtCBZ = regexp.MustCompile("(?i)\\.cbz$")
	reFileExtJPG = regexp.MustCompile("(?i)\\.jp(e|)g$")
	reFileExtPNG = regexp.MustCompile("(?i)\\.png$")
)

const (
	chExit = ":exit:"
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

func basicResizeBench(img image.Image) {
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
			log.Fatal(err)
		}
		log.Printf("scaling using %d takes %v time",
			i, time.Now().Sub(tp))

		err = imageSave(fmt.Sprintf("out-%d.png", i), img2)
		if err != nil {
			log.Fatal(err)
		}
	}
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
