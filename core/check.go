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

// DupInodeMap maps the inode with duplicate flag
type DupInodeMap map[int64]bool

// adjustable
var (
	// match image by exact match
	exactMatch = false
	// maximum acceptable image distance
	maxImageDist = 3
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
	dupInodeMap := DupInodeMap{}

	// detected dup groups
	groups := DupArchives{}

	for _, head := range archives {
		// fmt.Println("scan", archive.Name)
		// skip invalid
		if head.Inode == 0 {
			continue
		}
		// skip already found dup
		if dupInodeMap[head.Inode] {
			continue
		}

		dups := []*Archive{}

		if exactMatch {
			// find by exact image match archive
			dups = findExactMatch(head, archives, dupInodeMap)
		} else {
			// find by similar image match archive
			dups = findSimilarMatch(head, archives, dupInodeMap)
		}

		// skip no duplicate found
		if len(dups) == 0 {
			continue
		}

		// holds the dup and the head
		dup := &DupArchive{
			Head: head,
			Dups: dups,
		}

		groups = append(groups, dup)
		fmt.Printf("%d: (%d) %s\n", len(groups), head.Inode, head.Name)
		for i, d := range dup.Dups {
			fmt.Printf("  > %d (%d) %s\n", i, d.Inode, d.Name)
		}
		fmt.Printf("\n\n")
	}

	fmt.Printf("found %d dup groups\n", len(groups))
}

// todo fix bug for this function, every now and then it just grab too many not dup
func findExactMatch(head *Archive, archives Archives, dupInodeMap DupInodeMap) []*Archive {
	dups := []*Archive{}

	// loop all other archives to find
	for _, archive := range archives {
		// skip invalid
		if archive.Inode == 0 {
			continue
		}
		// skip itself
		if archive.Inode == head.Inode {
			continue
		}
		// skip if already mark as dup
		if dupInodeMap[archive.Inode] {
			continue
		}
		// skip if image length too different
		if math.Abs(float64(len(head.Images)-len(archive.Images))) > float64(maxArchiveLengthDiff) {
			continue
		}

		found := 0
		for _, headImage := range head.Images {
			for _, image := range archive.Images {
				if image.CRC32 == headImage.CRC32 &&
					image.DataSize == headImage.DataSize &&
					image.Height == headImage.Height &&
					image.Width == headImage.Width {
					found++
				}
			}
		}
		if len(head.Images)-found > maxArchiveLengthDiff {
			continue
		}

		dups = append(dups, archive)

		// set dup flag
		dupInodeMap[archive.Inode] = true
		dupInodeMap[head.Inode] = true
	}

	return dups
}

func findSimilarMatch(head *Archive, archives Archives, dupInodeMap DupInodeMap) []*Archive {
	dups := []*Archive{}

	// loop all other archives to find
	for _, archive := range archives {
		// skip invalid
		if archive.Inode == 0 {
			continue
		}
		// skip itself
		if archive.Inode == head.Inode {
			continue
		}
		// skip if already mark as dup
		if dupInodeMap[archive.Inode] {
			continue
		}
		// skip if no enough images to compare
		if len(head.Images) <= 5 {
			continue
		}
		// skip if no enough images to compare (b)
		if len(archive.Images) <= 5 {
			continue
		}
		// skip if image length too different
		if math.Abs(float64(len(head.Images)-len(archive.Images))) > float64(maxArchiveLengthDiff) {
			continue
		}

		// matching pHashes for similar match
		imgHeads := []uint64{}
		for i := 0; i < len(head.Images); i++ {
			// no blank page, all 0s
			if head.Images[i].PHash == 0 {
				continue
			}
			// need only 5
			if len(imgHeads) >= 5 {
				break
			}

			imgHeads = append(imgHeads, head.Images[i].PHash)
		}

		// score for keeping how many pHash match consecutive
		score := 0
		for i, image := range archive.Images {
			// dont go too far to save cpu cycle
			if i > 10 {
				break
			}
			// find dup
			for _, imgHead := range imgHeads {
				if calcDist(imgHead, image.PHash) <= maxImageDist {
					score++
				}
			}
		}

		// at least find x dup image before classify as dup archive
		if score >= minScore {
			dups = append(dups, archive)

			// set dup flag
			dupInodeMap[archive.Inode] = true
			dupInodeMap[head.Inode] = true
		}
	}

	return dups
}

func rmDup() {

}

func hostUI() {

}

// FindDup exec find duplicate archive
func FindDup(dir string, maxIDiff, maxADiff int, exMatch bool) error {
	archives, err := loadSums(dir)
	if err != nil {
		return err
	}

	maxImageDist = maxIDiff
	maxArchiveLengthDiff = maxADiff
	exactMatch = exMatch

	fmt.Printf("found %d txt\n", len(archives))

	findDup(archives)
	return nil
}
