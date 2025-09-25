/*
gura

Copyright (c) 2024-2025 Fyra Labs

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

// kudari: Fetch and handle downstream repository metadata
//
// This file `rpm.go` contains logic for fetching, parsing, and processing RPM repository metadata.
// It handles downloading and decoding repomd.xml and primary.xml files, decompressing them as needed,
// and updating the local package database with the latest package information.
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
	"github.com/terrapkg/gura/nobori"
	. "github.com/terrapkg/gura/repomd"
	"github.com/terrapkg/gura/util"
	"go.uber.org/zap"
)

// Fetch and decode the repomd.xml file from the specified repository URL
//
// On failure, return nil
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

// Fetch and decode primary.xml, handling compression as needed
//
// On failure, return nil
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

// Retrieve a sorted list of [rpmPackageXML] structs for each package in the primary.xml file
//
// The results are sent to the provided channel.
// This only obtains the per-arch package list as given in `fetch`.
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

// Fetch and Update package list in db
//
// Merge package list from all fetch URLs, and update the database.
func rpmFetch(repo db.Repo) {
	chans := []chan []RPMPackageXML{}
	for fetch := range strings.SplitSeq(repo.Fetch, "\n") {
		ch := make(chan []RPMPackageXML, 1)
		chans = append(chans, ch)
		go rpmEachFetch(repo, fetch, ch)
	}

	// perf: should be fine, bottleneck in network fetch, not local db
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
	updated, unchanged, lastIdx, newpkgs := 0, 0, 0, 0
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
			util.MaybeSuicide(l, "Meta.UnmarshalJSON", pkgs[n].Meta.UnmarshalJSON(rpm2MetaJSON(p)))
			updated++
		} else {
			p := &db.Pkg{
				Name:    p.Name,
				FullVer: rpmFullVer(p),
				Ver:     p.Version.Ver,
				Arch:    p.Arch,
				RepoID:  repo.ID,
				Meta:    rpm2MetaJSON(p),
			}
			util.Yeet(l, "cannot RegPkg", nobori.RegPkg(tx, p), zap.Any("pkg", p))
			newpkgs++
		}
	}
	var deletes []uuid.UUID
	for i, p := range pkgs {
		if !walked[i] {
			deletes = append(deletes, uuid.UUID(p.ID))
		}
	}
	tx.Delete(&db.Pkg{}, "id IN (?)", deletes)
	tx.Commit()
	l.Info("package update summary",
		zap.String("repoID", repo.ID),
		zap.Int("unchanged", unchanged),
		zap.Int("updated", updated),
		zap.Int("added", newpkgs),
		zap.Int("deleted", len(deletes)),
	)
}

// Full version string for a package, combining epoch, version, and release
func rpmFullVer(p RPMPackageXML) string {
	return fmt.Sprintf("%s:%s-%s", p.Version.Epoch, p.Version.Ver, p.Version.Rel)
}

// Compare two [RPMPackageXML] by name and architecture for sorting purposes
func rpmCompare(a, b RPMPackageXML) int {
	if cmp := strings.Compare(a.Name, b.Name); cmp != 0 {
		return cmp
	}
	return strings.Compare(a.Arch, b.Arch)
}

// Serialize all fields of [RPMPackageXML] except Name, Arch, and Version into JSON for storage in [db.Pkg.Meta].
func rpm2MetaJSON(p RPMPackageXML) []byte {
	bs, err := json.Marshal(RPMMeta{
		Checksum: p.Checksum,
		Packager: p.Packager,
		Url:      p.Url,
		Format:   p.Format,
	})
	util.MaybeSuicide(l, "cannot marshal meta", err, zap.Any("RPMPackageXML", bs))
	return bs
}
