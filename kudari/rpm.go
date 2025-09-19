// Fetch RPM repomd from a repository
package kudari

import (
	"compress/gzip"
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/klauspost/compress/zstd"
	"github.com/terrapkg/gura/db"
	"github.com/terrapkg/gura/util"
	"go.uber.org/zap"
)

type Repomd struct {
	XMLName  xml.Name `xml:"repomd"`
	Revision string   `xml:"revision"`
	Data     []struct {
		Type            string    `xml:"type,attr"`
		Checksum        Checksum  `xml:"checksum"`
		OpenChecksum    *Checksum `xml:"open-checksum,omitempty"`
		HeaderChecksum  *Checksum `xml:"header-checksum,omitempty"`
		Timestamp       int64     `xml:"timestamp"`
		Size            int64     `xml:"size"`
		OpenSize        *int64    `xml:"open-size,omitempty"`
		HeaderSize      *int64    `xml:"header-size,omitempty"`
		DatabaseVersion *int      `xml:"database_version,omitempty"`
		Location        struct {
			Href string `xml:"href,attr"`
		} `xml:"location"`
	} `xml:"data"`
}
type Checksum struct {
	Type  string `xml:"type,attr"`
	Value string `xml:"chardata"`
}

type PackageXML struct {
	Name    string `xml:"name"`
	Arch    string `xml:"arch"`
	Version struct {
		Epoch string `xml:"epoch,attr"`
		Ver   string `xml:"ver,attr"`
		Rel   string `xml:"rel,attr"`
	} `xml:"version"`
	Checksum Checksum `xml:"checksum"`
	Packager string   `xml:"packager"`
	Url      string   `xml:"url"`
	// time
	// size
	// location
	Format struct {
		License     *string    `xml:"license,omitempty"`
		Vendor      *string    `xml:"vendor,omitempty"`
		Group       *string    `xml:"group,omitempty"`
		Buildhost   *string    `xml:"buildhost,omitempty"`
		Sourcerpm   *string    `xml:"sourcerpm,omitempty"`
		Provides    []RPMEntry `xml:"provides>entry,omitempty"`
		Requires    []RPMEntry `xml:"requires>entry,omitempty"`
		Obsoletes   []RPMEntry `xml:"obsoletes>entry,omitempty"`
		Conflicts   []RPMEntry `xml:"conflicts>entry,omitempty"`
		Enhances    []RPMEntry `xml:"enhances>entry,omitempty"`
		Suggests    []RPMEntry `xml:"suggests>entry,omitempty"`
		Recommends  []RPMEntry `xml:"recommends>entry,omitempty"`
		Supplements []RPMEntry `xml:"supplements>entry,omitempty"`
	} `xml:"format"`
}
type RPMEntry struct {
	Name  string `xml:"name,attr"`
	Flags string `xml:"flags,attr"`
	Epoch string `xml:"epoch,attr"`
	Ver   string `xml:"ver,attr"`
	Rel   string `xml:"rel,attr"`
}

type PrimaryXML struct {
	Packages []PackageXML `xml:"package"`
}

// Obtain [Repomd] from a repository
func getRepomd(repoID, fetch string) *Repomd {
	resp, err := http.Get(fmt.Sprintf("%s/repodata/repomd.xml", fetch))
	if util.Yeet(l, "Failed to fetch repomd.xml", err, zap.String("repoID", repoID)) {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		l.Error("repomd HTTP error", zap.String("repoID", repoID), zap.Int("status code", resp.StatusCode))
		return nil
	}
	l.Info("decoding repomd.xml", zap.String("repoID", repoID))

	decoder := xml.NewDecoder(resp.Body)
	var repomd Repomd
	if util.Yeet(l, "Failed to parse repomd.xml", decoder.Decode(&repomd)) {
		return nil
	}
	l.Info("repomd.xml decoded successfully", zap.String("repoID", repoID))
	return &repomd
}

// Obtain [PrimaryXML] from a repository
func getPrimary(repoID, fetch string, repomd Repomd) (primary *PrimaryXML) {
	var primaryLocation string
	var compression string

	for _, data := range repomd.Data {
		if data.Type == "primary" {
			href := data.Location.Href
			switch {
			case strings.HasSuffix(href, ".xml.zst"):
				primaryLocation = href
				compression = "zst"
			case strings.HasSuffix(href, ".xml.gz"):
				primaryLocation = href
				compression = "gz"
			default:
				l.Error("Unsupported compression type for primary.xml", zap.String("repoID", repoID), zap.String("href", href))
				return
			}
		}
	}

	primaryURL := fmt.Sprintf("%s/%s", fetch, primaryLocation)
	l.Info("fetching primary.xml", zap.String("repoID", repoID), zap.String("compression", compression))
	resp, err := http.Get(primaryURL)
	if util.Yeet(l, "Failed to fetch primary.xml", err, zap.String("repoID", repoID), zap.String("compression", compression)) {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		l.Error("HTTP error fetching primary.xml", zap.String("repoID", repoID), zap.String("compression", compression), zap.Int("statusCode", resp.StatusCode))
		return
	}

	var xmlReader interface{ Read([]byte) (int, error) }
	switch compression {
	case "gz":
		gzReader, err := gzip.NewReader(resp.Body)
		if util.Yeet(l, "Failed to create gzip reader", err, zap.String("repoID", repoID), zap.String("compression", compression)) {
			return
		}
		defer gzReader.Close()
		xmlReader = gzReader
	case "zst":
		zstdDecoder, err := zstd.NewReader(resp.Body)
		if util.Yeet(l, "Failed to create zstd reader", err, zap.String("repoID", repoID), zap.String("compression", compression)) {
			return
		}
		defer zstdDecoder.Close()
		xmlReader = zstdDecoder
	default:
		l.Panic("Unknown compression type for primary.xml", zap.String("repoID", repoID), zap.String("compression", compression))
		return
	}

	decoder := xml.NewDecoder(xmlReader)
	primary = &PrimaryXML{}
	if util.Yeet(l, "Failed to parse primary.xml", decoder.Decode(primary), zap.String("repoID", repoID), zap.String("compression", compression)) {
		return
	}

	l.Info("primary.xml decoded successfully", zap.String("repoID", repoID))
	return
}

// Obtain a sorted list of [PackageXML] structs for each package in the primary.xml file.
//
// Sorting is defined by [Compare].
func eachFetch(repo db.Repo, fetch string, ch chan []PackageXML) {
	repomd := getRepomd(repo.ID, fetch)
	if repomd == nil {
		close(ch)
		return
	}
	primary := getPrimary(repo.ID, fetch, *repomd)
	if primary == nil {
		close(ch)
		return
	}
	util.InsertionSort(&primary.Packages, Compare) // ensure sorted, though usually already sorted
	ch <- primary.Packages
	close(ch)
}

func rpmFetch(repo db.Repo) {
	chans := []chan []PackageXML{}
	for fetch := range strings.SplitSeq(repo.Fetch, "\n") {
		ch := make(chan []PackageXML, 1)
		chans = append(chans, ch)
		go eachFetch(repo, fetch, ch)
	}
	var pkgs []db.Pkg
	if util.Yeet(l, "Failed to list packages", db.DB.Where("repo_id = ? AND deleted_at IS NULL", repo.ID).Order("name, arch").Find(&pkgs).Error) {
		return
	}
	allSlices := [][]PackageXML{}
	for _, ch := range chans {
		local_packages, ok := <-ch
		if !ok {
			log.Printf("[%s] stop, couldn't fetch primary", repo.ID)
			return
		}
		allSlices = append(allSlices, local_packages)
	}
	packages := util.MergeSortedDedup(allSlices, Compare)
	log.Printf("[%s] processing %d packages", repo.ID, len(packages))
	var newpkgs []*db.Pkg
	updated := 0
	unchanged := 0
	lastIdx := 0
	walked := make([]bool, len(pkgs))
	tx := db.DB.Begin()
	for _, p := range packages {
		if util.SortedContSearch(pkgs, p, func(a db.Pkg, b PackageXML) int {
			return Compare(PackageXML{
				Name: a.Name,
				Arch: a.Arch,
			}, b)
		}, &lastIdx) {
			n := lastIdx
			lastIdx++ // next search should start from the next index
			fullver := fullVer(p)
			walked[n] = true
			if fullver == pkgs[n].FullVer {
				unchanged++
				continue
			}
			pkgs[n].FullVer = fullver
			pkgs[n].Ver = p.Version.Ver
			tx.Save(&pkgs[n])
			updated++
		} else {
			newpkgs = append(newpkgs, &db.Pkg{
				Name:    p.Name,
				FullVer: fullVer(p),
				Ver:     p.Version.Ver,
				Arch:    p.Arch,
				RepoID:  repo.ID,
			})
		}
	}
	var deletes []uuid.UUID
	for i, p := range pkgs {
		if !walked[i] {
			deletes = append(deletes, uuid.UUID(p.ID))
		}
	}
	tx.Delete(&db.Pkg{}, "id IN (?)", deletes)
	if newpkgs != nil {
		tx.CreateInBatches(newpkgs, 5000)
	}
	tx.Commit()
	log.Printf("[%s] unchanged=%d, updated=%d, added=%d, deleted=%d", repo.ID, unchanged, updated, len(newpkgs), len(deletes))
}

func fullVer(p PackageXML) string {
	return fmt.Sprintf("%s:%s-%s", p.Version.Epoch, p.Version.Ver, p.Version.Rel)
}

func Compare(a, b PackageXML) int {
	if cmp := strings.Compare(a.Name, b.Name); cmp != 0 {
		return cmp
	}
	return strings.Compare(a.Arch, b.Arch)
}
