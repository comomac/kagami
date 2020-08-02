package core

import (
	"fmt"
	"io/ioutil"
	"math"
	"math/bits"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
)

// check and look for archives with duplicate images

// Archive holds the images
type Archive struct {
	Name   string      // full zip file path
	MTime  time.Time   // zip file modified time
	Inode  int64       // zip file inode
	Images []*ZipImage // metadata for images
	Exact  bool        // exact match to head archive
}

// Archives list of image archives
type Archives []*Archive

// DupArchive duplicate archive
type DupArchive struct {
	Head *Archive
	Dups []*Archive
}

// DupArchives list of duplicate archives
type DupArchives []*DupArchive

// adjustable
var (
	// maximum acceptable image distance
	maxDist = 10
	// minimum score to accept as archive match
	minScore = 4
	// maximum acceptable images length between archive
	maxArchiveLengthDiff = 10
)

func loadSums(dir string) (Archives, error) {
	archives := Archives{}

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
		if !strings.HasSuffix(info.Name(), ".txt") {
			return nil
		}

		archive, err := loadSum(file)
		if err != nil {
			fmt.Println("err loadSum", file)
			return nil
		}
		if archive.Inode == 0 {
			return fmt.Errorf("ino is 0")
		}

		archives = append(archives, archive)

		return nil
	})
	if err != nil {
		return nil, err
	}

	return archives, nil
}

func loadSum(file string) (*Archive, error) {
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(b), "\n")

	sIno := strings.ReplaceAll(filepath.Base(file), ".txt", "")
	ino, err := strconv.Atoi(sIno)
	if err != nil {
		return nil, err
	}

	archive := &Archive{
		Inode: int64(ino),
	}

	imageNth := 0
	for _, line := range lines {
		line2 := strings.TrimSpace(line)
		if strings.HasPrefix(line2, "#") {
			// find out archive file name
			if strings.HasPrefix(line2, "# file: ") {
				archive.Name = strings.ReplaceAll(line2, "# file: ", "")
			}
			continue
		}

		// make sure line has at least 47 chars
		if len(line) < 47 {
			continue
		}

		left := line[0:44]
		right := line[46:]

		zz := ZipImage{
			Name: right,
			Nth:  imageNth,
		}
		_, err := fmt.Sscanf(left, "%8X %d %d %d %16X", &zz.CRC32, &zz.DataSize, &zz.Width, &zz.Height, &zz.PHash)
		if err != nil {
			fmt.Println("err", err, file)
			spew.Dump(left)
			continue
		}
		imageNth++

		archive.Images = append(archive.Images, &zz)
	}

	return archive, nil
}

func calcDist(a, b uint64) int {
	// xor
	c := a ^ b

	return bits.OnesCount64(c)
}

func findDup(archives Archives) {
	// file inodes that been found to be dup
	inodes := map[int64]bool{}

	// detected dup groups
	groups := DupArchives{}

	for _, head := range archives {
		// fmt.Println("scan", archive.Name)
		// skip invalid
		if head.Inode == 0 {
			continue
		}
		// skip already found dup
		if inodes[head.Inode] {
			continue
		}

		// find by similar archive
		dups := findSimilarMatch(head, archives)

		// skip no duplicate found
		if len(dups) == 0 {
			continue
		}

		// holds the dup and the head
		dup := &DupArchive{
			Head: head,
			Dups: dups,
		}

		// set dup flag
		for _, arc := range dups {
			inodes[arc.Inode] = true
		}
		// set itself to be dup
		inodes[head.Inode] = true

		groups = append(groups, dup)
		fmt.Printf("%d: (%d) %s\n", len(groups), head.Inode, head.Name)
		for i, d := range dup.Dups {
			fmt.Printf("  > %d (%d) %s\n", i, d.Inode, d.Name)
		}
		fmt.Printf("\n\n")
	}

	fmt.Printf("found %d dup groups\n", len(groups))
}

func isExactMatch() {

}

func findSimilarMatch(head *Archive, archives Archives) []*Archive {
	dups := []*Archive{}

	// loop all other archives to find
	for _, archive2 := range archives {
		// skip invalid
		if archive2.Inode == 0 {
			continue
		}
		// skip itself
		if archive2.Inode == head.Inode {
			continue
		}
		// skip if image length too different
		if math.Abs(float64(len(head.Images)-len(archive2.Images))) > float64(maxArchiveLengthDiff) {
			continue
		}
		// skip if no enough images to compare
		if len(head.Images) <= 5 {
			continue
		}
		// skip if no enough images to compare (b)
		if len(archive2.Images) <= 5 {
			continue
		}

		// matching pHashes for similar match
		imgHeads := []uint64{}
		for i := 0; i < len(head.Images); i++ {
			if head.Images[i].PHash == 0 {
				// no blank page, all 0s
				continue
			}
			if len(imgHeads) >= 5 {
				// need only 5
				break
			}

			imgHeads = append(imgHeads, head.Images[i].PHash)
		}

		// score for keeping how many pHash match consecutive
		score := 0
		for i, image := range archive2.Images {
			// dont go too far to save cpu cycle
			if i > 10 {
				break
			}
			// find dup
			for _, imgHead := range imgHeads {
				if calcDist(imgHead, image.PHash) <= maxDist {
					score++
				}
			}
		}

		// at least find x dup image before classify as dup archive
		if score >= minScore {
			dups = append(dups, archive2)
		}
	}

	return dups
}

func rmDup() {

}

func hostUI() {

}

// FindDup exec find duplicate archive
func FindDup(dir string, id, ad int) error {
	archives, err := loadSums(dir)
	if err != nil {
		return err
	}

	maxDist = id
	maxArchiveLengthDiff = ad

	fmt.Printf("found %d txt\n", len(archives))

	findDup(archives)
	return nil
}
