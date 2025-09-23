// Fetch RPM repomd from a repository
package kudari

import (
	"compress/gzip"
	"encoding/json"
	"encoding/xml"
	"fmt"

	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/klauspost/compress/zstd"
	"github.com/terrapkg/gura/db"
	"github.com/terrapkg/gura/util"
	"go.uber.org/zap"
)

type RPMRepomd struct {
	XMLName  xml.Name `xml:"repomd"`
	Revision string   `xml:"revision"`
	Data     []struct {
		Type            string       `xml:"type,attr"`
		Checksum        RPMChecksum  `xml:"checksum"`
		OpenChecksum    *RPMChecksum `xml:"open-checksum,omitempty"`
		HeaderChecksum  *RPMChecksum `xml:"header-checksum,omitempty"`
		Timestamp       int64        `xml:"timestamp"`
		Size            int64        `xml:"size"`
		OpenSize        *int64       `xml:"open-size,omitempty"`
		HeaderSize      *int64       `xml:"header-size,omitempty"`
		DatabaseVersion *int         `xml:"database_version,omitempty"`
		Location        struct {
			Href string `xml:"href,attr"`
		} `xml:"location"`
	} `xml:"data"`
}
type RPMChecksum struct {
	Type  string `xml:"type,attr"`
	Value string `xml:"chardata"`
}

type RPMPackageXML struct {
	Name    string `xml:"name"`
	Arch    string `xml:"arch"`
	Version struct {
		Epoch string `xml:"epoch,attr"`
		Ver   string `xml:"ver,attr"`
		Rel   string `xml:"rel,attr"`
	} `xml:"version"`
	Checksum RPMChecksum `xml:"checksum"`
	Packager string      `xml:"packager"`
	Url      string      `xml:"url"`
	// time
	// size
	// location
	Format RPMFormat `xml:"format"`
}
type RPMFormat struct {
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
}
type RPMEntry struct {
	Name  string `xml:"name,attr"`
	Flags string `xml:"flags,attr"`
	Epoch string `xml:"epoch,attr"`
	Ver   string `xml:"ver,attr"`
	Rel   string `xml:"rel,attr"`
}

type RPMPrimaryXML struct {
	Packages []RPMPackageXML `xml:"package"`
}

// Obtain [Repomd] from a repository
func rpmGetRepomd(repoID, fetch string) *RPMRepomd {
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
	var repomd RPMRepomd
	if util.Yeet(l, "Failed to parse repomd.xml", decoder.Decode(&repomd)) {
		return nil
	}
	l.Info("repomd.xml decoded successfully", zap.String("repoID", repoID))
	return &repomd
}

// Obtain [RPMPrimaryXML] from a repository
func rpmGetPrimary(repoID, fetch string, repomd RPMRepomd) (primary *RPMPrimaryXML) {
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
	primary = &RPMPrimaryXML{}
	if util.Yeet(l, "Failed to parse primary.xml", decoder.Decode(primary), zap.String("repoID", repoID), zap.String("compression", compression)) {
		return
	}

	l.Info("primary.xml decoded successfully", zap.String("repoID", repoID))
	return
}

// Obtain a sorted list of [rpmPackageXML] structs for each package in the primary.xml file.
//
// Sorting is defined by [rpmCompare].
func rpmEachFetch(repo db.Repo, fetch string, ch chan []RPMPackageXML) {
	repomd := rpmGetRepomd(repo.ID, fetch)
	if repomd == nil {
		close(ch)
		return
	}
	primary := rpmGetPrimary(repo.ID, fetch, *repomd)
	if primary == nil {
		close(ch)
		return
	}
	util.InsertionSort(&primary.Packages, rpmCompare) // ensure sorted, though usually already sorted
	ch <- primary.Packages
	close(ch)
}

func rpmFetch(repo db.Repo) {
	chans := []chan []RPMPackageXML{}
	for fetch := range strings.SplitSeq(repo.Fetch, "\n") {
		ch := make(chan []RPMPackageXML, 1)
		chans = append(chans, ch)
		go rpmEachFetch(repo, fetch, ch)
	}
	var pkgs []db.Pkg
	if util.Yeet(l, "Failed to list packages", db.DB.Where("repo_id = ? AND deleted_at IS NULL", repo.ID).Order("name, arch").Find(&pkgs).Error) {
		return
	}
	allSlices := [][]RPMPackageXML{}
	for _, ch := range chans {
		local_packages, ok := <-ch
		if !ok {
			l.Error("stop, couldn't fetch primary", zap.String("repoID", repo.ID))
			return
		}
		allSlices = append(allSlices, local_packages)
	}
	packages := util.MergeSortedDedup(allSlices, rpmCompare)
	l.Info("processing packages", zap.String("repoID", repo.ID), zap.Int("count", len(packages)))
	var newpkgs []*db.Pkg
	updated := 0
	unchanged := 0
	lastIdx := 0
	walked := make([]bool, len(pkgs))
	tx := db.DB.Begin()
	for _, p := range packages {
		if util.SortedContSearch(pkgs, p, func(a db.Pkg, b RPMPackageXML) int {
			return rpmCompare(RPMPackageXML{
				Name: a.Name,
				Arch: a.Arch,
			}, b)
		}, &lastIdx) {
			n := lastIdx
			lastIdx++ // next search should start from the next index
			fullver := rpmFullVer(p)
			walked[n] = true
			if fullver == pkgs[n].FullVer {
				unchanged++
				continue
			}
			pkgs[n].FullVer = fullver
			pkgs[n].Ver = p.Version.Ver
			meta, _ := rpm2MetaJSON(p)
			pkgs[n].Meta = meta
			tx.Save(&pkgs[n])
			updated++
		} else {
			meta, _ := rpm2MetaJSON(p)
			newpkgs = append(newpkgs, &db.Pkg{
				Name:    p.Name,
				FullVer: rpmFullVer(p),
				Ver:     p.Version.Ver,
				Arch:    p.Arch,
				RepoID:  repo.ID,
				Meta:    meta,
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
	l.Info("package update summary",
		zap.String("repoID", repo.ID),
		zap.Int("unchanged", unchanged),
		zap.Int("updated", updated),
		zap.Int("added", len(newpkgs)),
		zap.Int("deleted", len(deletes)),
	)
}

func rpmFullVer(p RPMPackageXML) string {
	return fmt.Sprintf("%s:%s-%s", p.Version.Epoch, p.Version.Ver, p.Version.Rel)
}

func rpmCompare(a, b RPMPackageXML) int {
	if cmp := strings.Compare(a.Name, b.Name); cmp != 0 {
		return cmp
	}
	return strings.Compare(a.Arch, b.Arch)
}

// packageMetaJSON serializes all PackageXML fields except Name, Arch, and Version into JSON
func rpm2MetaJSON(p RPMPackageXML) ([]byte, error) {
	meta := struct {
		Checksum RPMChecksum
		Packager string
		Url      string
		Format   RPMFormat
	}{
		Checksum: p.Checksum,
		Packager: p.Packager,
		Url:      p.Url,
		Format:   p.Format,
	}
	return json.Marshal(meta)
}
