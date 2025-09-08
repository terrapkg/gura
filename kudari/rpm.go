// Fetch RPM repomd from a repository
package kudari

import (
	"compress/gzip"
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/klauspost/compress/zstd"
	"github.com/terrapkg/gura/db"
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
}

type PrimaryXML struct {
	Packages []PackageXML `xml:"package"`
}

func getRepomd(repoID, fetch string) *Repomd {
	resp, err := http.Get(fmt.Sprintf("%s/repodata/repomd.xml", fetch))
	if err != nil {
		log.Printf("[%s] Failed to fetch repomd.xml: %v", repoID, err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[%s] HTTP error repomd: %d", repoID, resp.StatusCode)
		return nil
	}
	log.Printf("[%s] decoding repomd.xml", repoID)

	decoder := xml.NewDecoder(resp.Body)
	var repomd Repomd
	if err := decoder.Decode(&repomd); err != nil {
		log.Printf("[%s] Failed to parse repomd.xml: %v", repoID, err)
		return nil
	}
	log.Printf("[%s] repomd.xml decoded successfully", repoID)
	return &repomd
}

func getPrimary(repoID, fetch string, repomd Repomd) (primary *PrimaryXML) {
	var primaryLocation string
	var compression string

	// Prefer zst over zck over gz
	for _, data := range repomd.Data {
		if data.Type == "primary" {
			href := data.Location.Href
			switch {
				case strings.HasSuffix(href, ".xml.zst"):
					primaryLocation = href
					compression = "zst"
				// case strings.HasSuffix(href, ".xml.zck"):
				// 	primaryLocation = href
				// 	compression = "zck"
				case strings.HasSuffix(href, ".xml.gz"):
					primaryLocation = href
					compression = "gz"
				default:
					log.Printf("[%s] Unsupported compression type for primary.xml: %s", repoID, href)
			}
		}
	}
	if primaryLocation == "" {
		log.Printf("[%s] No 'primary' data found in repomd.xml", repoID)
		return
	}

	primaryURL := fmt.Sprintf("%s/%s", fetch, primaryLocation)
	log.Printf("[%s] fetching primary.xml.%s", repoID, compression)
	resp, err := http.Get(primaryURL)
	if err != nil {
		log.Printf("[%s] Failed to fetch primary.xml.%s: %v", repoID, compression, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[%s] HTTP error fetching primary.xml.%s: %d", repoID, compression, resp.StatusCode)
		return
	}

	var xmlReader any
	switch compression {
	case "gz":
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			log.Printf("[%s] Failed to create gzip reader: %v", repoID, err)
			return
		}
		defer gzReader.Close()
		xmlReader = gzReader
	case "zst":
		zstdDecoder, err := zstd.NewReader(resp.Body)
		if err != nil {
			log.Printf("[%s] Failed to create zstd reader: %v", repoID, err)
			return
		}
		defer zstdDecoder.Close()
		xmlReader = zstdDecoder
	default:
		log.Printf("[%s] Unknown compression type for primary.xml: %s", repoID, compression)
		return
	}

	decoder := xml.NewDecoder(xmlReader.(interface{ Read([]byte) (int, error) }))
	primary = &PrimaryXML{}
	if err := decoder.Decode(primary); err != nil {
		log.Printf("[%s] Failed to parse primary.xml: %v", repoID, err)
		return
	}

	log.Printf("[%s] primary.xml decoded successfully", repoID)
	return
}

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
	ch <- primary.Packages
	close(ch)
}

func rpmFetch(repo db.Repo) {
	chans := []chan []PackageXML{}
	for fetch := range strings.SplitSeq(repo.Fetch, "\n") {
		ch := make(chan []PackageXML)
		chans = append(chans, ch)
		go eachFetch(repo, fetch, ch)
	}
	var pkgs []db.Pkg
	if err := db.DB.Where("repo_id = ? AND deleted_at IS NULL", repo.ID).Order("name, arch").Find(&pkgs).Error; err != nil {
		log.Printf("[%s] Failed to list packages: %v", repo.ID, err)
		return
	}
	packages := []PackageXML{}
	seen := make(map[string]struct{})
	for _, ch := range chans {
		local_packages, ok := <-ch
		if !ok {
			continue
		}
		for _, pkg := range local_packages {
			key := pkg.Name + "|" + pkg.Arch
			if _, exists := seen[key]; !exists {
				seen[key] = struct{}{}
				packages = append(packages, pkg)
			}
		}
	}
	log.Printf("[%s] processing %d packages", repo.ID, len(packages))
	var newpkgs []*db.Pkg
	updated := 0
	unchanged := 0
	tx := db.DB.Begin()
	for _, p := range packages {
		n, found := slices.BinarySearchFunc(pkgs, db.Pkg {
			Name: p.Name,
			Arch: p.Arch,
		}, func (a, b db.Pkg) (i int) {
			if i = strings.Compare(a.Name, b.Name); i == 0 {
				return strings.Compare(a.Arch, b.Arch)
			}
			return i
		})
		if found {
			fullver := fullVer(p)
			if fullver == pkgs[n].FullVer {
				pkgs = slices.Delete(pkgs, n, n+1)
				unchanged++
				continue
			}
			pkgs[n].FullVer = fullver
			pkgs[n].Ver = p.Version.Ver
			tx.Save(&pkgs[n])
			pkgs = slices.Delete(pkgs, n, n+1)
			updated++
		} else {
			newpkgs = append(newpkgs, &db.Pkg {
				Name: p.Name,
				FullVer: fullVer(p),
				Ver: p.Version.Ver,
				Arch: p.Arch,
				RepoID: repo.ID,
			})
		}
	}
	var pkg_ids []uuid.UUID
	for _, p := range pkgs {
		pkg_ids = append(pkg_ids, p.ID)
	}
	tx.Delete(&db.Pkg{}, "id IN (?)", pkg_ids)
	if newpkgs != nil {
		tx.CreateInBatches(newpkgs, 5000)
	}
	tx.Commit()
	log.Printf("[%s] unchanged=%d, updated=%d, added=%d, deleted=%d", repo.ID, unchanged, updated, len(newpkgs), len(pkgs))
}

func fullVer(p PackageXML) string {
	return fmt.Sprintf("%s:%s-%s", p.Version.Epoch, p.Version.Ver, p.Version.Rel)
}
