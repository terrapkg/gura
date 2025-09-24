// nobori: upstream metadata fetching from various streams
//
// `strm.go`: determine stream of a package.
package nobori

import (
	"encoding/json"
	"errors"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/mdobak/go-xerrors"
	"github.com/terrapkg/gura/db"
	"github.com/terrapkg/gura/repomd"
	"github.com/terrapkg/gura/util"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var newStrmHdlrs = []func(p db.Pkg, url string, urls []string) bool{
	func(p db.Pkg, url string, urls []string) bool {
		if !strings.HasPrefix(url, "github.com/") {
			return false
		}
		strm := GhStrmHdlr(p, url)
		strm.Mirrors = strings.Join(urls, ",")
		db.DB.Save(strm)
		return true
	},
}

var mangleUrlRegex = regexp.MustCompile(`^(https?://[^/]+)(/.*)$`)

func mangleUrl(url string) (new string) {
	match := mangleUrlRegex.FindSubmatch([]byte(url))
	if match == nil {
		return url
	}
	return string(match[1])
}

// Register package in db
func RegPkg(p db.Pkg) error {
	if p.StreamID != nil {
		return db.DB.Save(p).Error
	}
	url_ch := make(chan string)
	UpTrace(p, url_ch)
	close(url_ch)
	urls := map[string]struct{}{}
	var strm *db.Stream
recv:
	url, ok := <-url_ch
	if !ok {
		db.DB.Save(p)
		return handleNewStrm(p, slices.Collect(maps.Keys(urls)))
	}
	url = mangleUrl(url)
	urls[url] = struct{}{}
	if e := db.DB.Find(&strm, "? IN STRING_TO_ARRAY(mirrors)", url).Error; e != nil {
		if errors.Is(e, gorm.ErrRecordNotFound) {
			goto recv
		}
		return xerrors.Newf("can't find stream in db (pkgid %s): %w", p.ID.String(), e)
	}

	for url := range strings.SplitSeq(strm.Mirrors, ",") {
		urls[url] = struct{}{}
	}
	for url, ok := <-url_ch; ok; {
		urls[url] = struct{}{}
	}

	strm.Mirrors = strings.Join(slices.Collect(maps.Keys(urls)), ",")
	p.StreamID = &strm.ID
	if err := db.DB.Save(p).Error; err != nil {
		return xerrors.Newf("can't save package in db (pkgid %s): %w", p.ID.String(), err)
	}
	if err := db.DB.Save(strm).Error; err != nil {
		return xerrors.Newf("can't save stream in db (strmid %s): %w", strm.ID.String(), err)
	}
	return nil
}

func handleNewStrm(p db.Pkg, urls []string) error {
	for _, hdlr := range newStrmHdlrs {
		for _, url := range urls {
			if hdlr(p, url, urls) {
				return nil
			}
		}
	}
	l.Debug("didn't find any existing strm, and no strm supported", zap.String("pkgid", p.ID.String()))
	return db.DB.Save(p).Error
}

func UpTrace(p db.Pkg, url_ch chan string) {
	switch p.Repo.Type {
	case db.Rpm:
		rpmTrace(p, url_ch)
	default:
		l.DPanic("unreachable in uptrace")
	}
}

func rpmTrace(p db.Pkg, url_ch chan string) {
	bs, err := p.Meta.MarshalJSON()
	if util.Yeet(l, "error marshaling package metadata", err, zap.String("id", p.ID.String())) {
		l.DPanic("DPanic")
		return
	}

	var meta repomd.RPMMeta
	json.Unmarshal(bs, &meta)
	url_ch <- meta.Url
}
