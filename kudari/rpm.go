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

func getRepomd(repo db.Repo) *Repomd {
	resp, err := http.Get(fmt.Sprintf("%s/repodata/repomd.xml", repo.Fetch))
	if err != nil {
		log.Printf("[%s] Failed to fetch repomd.xml: %v", repo.ID, err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[%s] HTTP error repomd: %d", repo.ID, resp.StatusCode)
		return nil
	}
	log.Printf("[%s] decoding repomd.xml", repo.ID)

	decoder := xml.NewDecoder(resp.Body)
	var repomd Repomd
	if err := decoder.Decode(&repomd); err != nil {
		log.Printf("[%s] Failed to parse repomd.xml: %v", repo.ID, err)
		return nil
	}
	log.Printf("[%s] repomd.xml decoded successfully", repo.ID)
	return &repomd
}

func getPrimary(repo db.Repo, repomd Repomd) (primary *PrimaryXML) {
	var primaryLocation string
	for _, data := range repomd.Data {
		if data.Type == "primary" {
			primaryLocation = data.Location.Href
			break
		}
	}
	if primaryLocation == "" {
		log.Printf("[%s] No 'primary' data found in repomd.xml", repo.ID)
		return
	}

	primaryURL := fmt.Sprintf("%s/%s", repo.Fetch, primaryLocation)
	log.Printf("[%s] fetching primary.xml.gz", repo.ID)
	resp, err := http.Get(primaryURL)
	if err != nil {
		log.Printf("[%s] Failed to fetch primary.xml.gz: %v", repo.ID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[%s] HTTP error fetching primary.xml.gz: %d", repo.ID, resp.StatusCode)
		return
	}

	gzReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		log.Printf("[%s] Failed to create gzip reader: %v", repo.ID, err)
		return
	}
	defer gzReader.Close()

	decoder := xml.NewDecoder(gzReader)
	primary = &PrimaryXML{}
	if err := decoder.Decode(primary); err != nil {
		log.Printf("[%s] Failed to parse primary.xml: %v", repo.ID, err)
		return
	}

	log.Printf("[%s] primary.xml decoded successfully", repo.ID)
	return
}

func rpmFetch(repo db.Repo) {
	repomd := getRepomd(repo)
	if repomd == nil {
		return
	}
	primary := getPrimary(repo, *repomd)
	if primary == nil {
		return
	}
	log.Printf("[%s] adding %d packages to db", repo.ID, len(primary.Packages))
	var pkgs []db.Pkg
	if err := db.DB.Where("repo_id = ? AND deleted_at IS NULL", repo.ID).Order("name, arch").Find(&pkgs).Error; err != nil {
		log.Printf("[%s] Failed to list packages: %v", repo.ID, err)
		return
	}
	var newpkgs []*db.Pkg
	updated := 0
	unchanged := 0
	tx := db.DB.Begin()
	for _, p := range primary.Packages {
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
		tx.Create(newpkgs)
	}
	tx.Commit()
	log.Printf("[%s] unchanged=%d, updated=%d, added=%d, deleted=%d", repo.ID, unchanged, updated, len(newpkgs), len(pkgs))
}

func fullVer(p PackageXML) string {
	return fmt.Sprintf("%s:%s-%s", p.Version.Epoch, p.Version.Ver, p.Version.Rel)
}
