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

// nobori: upstream metadata fetching from various streams
//
// `strm.go`: determine stream of a package.
package nobori

import (
	"errors"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/mdobak/go-xerrors"
	"github.com/terrapkg/gura/db"
	"github.com/terrapkg/gura/util"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var newStrmHdlrs = []func(p *db.Pkg, url string, urls []string) bool{
	func(p *db.Pkg, url string, urls []string) bool {
		if !strings.HasPrefix(url, "github.com/") {
			return false
		}
		l.Debug("strm hdl github", zap.String("p", p.ID.String()))
		strm := GhStrmHdlr(*p, url)
		if strm == nil {
			return false
		}
		db.DB.Save(strm) // to make sure we can see this later, don't use dbx
		db.DB.Save(util.SliceMap(urls, func(url string) db.StreamMirror {
			return db.StreamMirror{
				StreamID: strm.ID,
				Mirror:   url,
			}
		}))
		p.StreamID = &strm.ID
		return true
	},
}

var mangleUrlRegex = regexp.MustCompile(`^(https?://)(.+)$`)

func mangleUrl(url string) (new string) {
	match := mangleUrlRegex.FindSubmatch([]byte(url))
	if match == nil {
		return url
	}
	return string(match[2])
}

// Register package in db
//
// dbx is used to save only packages.
func RegPkg(dbx *gorm.DB, p *db.Pkg) error {
	l.Debug("RegPkg", zap.String("pkgname", p.Name))
	if p.StreamID != nil {
		return dbx.Save(p).Error
	}
	url_ch := make(chan string)
	go UpTrace(*p, url_ch)
	urls := map[string]struct{}{}
	var mirrors []db.StreamMirror
	for url := range url_ch {
		url = mangleUrl(url)
		urls[url] = struct{}{}
		if e := dbx.Find(&mirrors, "stream_id = (SELECT stream_id FROM stream_mirrors WHERE mirror = ?)", strings.TrimSuffix(url, "/")).Error; e != nil {
			if errors.Is(e, gorm.ErrRecordNotFound) {
				continue
			}
			return xerrors.Newf("can't find StreamMirror in db (pkgid %s): %w", p.ID.String(), e)
		}
		if len(mirrors) == 0 {
			continue
		}
		return handleExistingStrm(urls, mirrors, url_ch, p, dbx)
	}
	dbx.Save(p)
	return handleNewStrm(dbx, p, slices.Collect(maps.Keys(urls)))
}

func handleExistingStrm(urls map[string]struct{}, mirrors []db.StreamMirror, url_ch chan string, p *db.Pkg, dbx *gorm.DB) error {
	util.SliceEach(mirrors, func(m db.StreamMirror) { urls[m.Mirror] = struct{}{} })
	var new_mirrors []db.StreamMirror
	for url := range url_ch {
		if _, has := urls[url]; !has {
			new_mirrors = append(new_mirrors, db.StreamMirror{
				StreamID: mirrors[0].StreamID,
				Mirror:   url,
			})
			urls[url] = struct{}{}
		}
	}
	p.StreamID = &mirrors[0].StreamID
	if err := dbx.Save(p).Error; err != nil {
		return xerrors.Newf("can't save package in db (pkgid %s): %w", p.ID.String(), err)
	}
	if len(new_mirrors) != 0 {
		if err := db.DB.Save(new_mirrors).Error; err != nil {
			return xerrors.Newf("can't save mirrors in db (strmid %s): %w", mirrors[0].StreamID.String(), err)
		}
	}
	l.Debug("reg success", zap.String("pkgid", p.ID.String()))
	return nil
}

func handleNewStrm(dbx *gorm.DB, p *db.Pkg, urls []string) error {
	l.Debug("new strm", zap.String("ID", p.ID.String()), zap.Strings("urls", urls))
	for _, hdlr := range newStrmHdlrs {
		for _, url := range urls {
			if hdlr(p, url, urls) {
				l.Debug("new strm success", zap.String("pkgname", p.Name), zap.String("url", url))
				return nil
			}
		}
	}
	l.Debug("didn't find any existing strm, and no strm supported", zap.String("pkgname", p.Name), zap.Strings("urls", urls))
	return dbx.Save(p).Error
}
