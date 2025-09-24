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
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var newStrmHdlrs = []func(db *gorm.DB, p *db.Pkg, url string, urls []string) bool{
	func(dbx *gorm.DB, p *db.Pkg, url string, urls []string) bool {
		if !strings.HasPrefix(url, "github.com/") {
			return false
		}
		l.Debug("strm hdl github", zap.String("p", p.ID.String()))
		strm := GhStrmHdlr(*p, url)
		strm.Mirrors = strings.Join(urls, ",")
		dbx.Save(strm)
		p.StreamID = &strm.ID
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
func RegPkg(dbx *gorm.DB, p *db.Pkg) error {
	if p.StreamID != nil {
		return dbx.Save(p).Error
	}
	url_ch := make(chan string)
	UpTrace(*p, url_ch)
	close(url_ch)
	urls := map[string]struct{}{}
	var strm *db.Stream
recv:
	url, ok := <-url_ch
	if !ok {
		dbx.Save(p)
		return handleNewStrm(dbx, p, slices.Collect(maps.Keys(urls)))
	}
	url = mangleUrl(url)
	urls[url] = struct{}{}
	if e := dbx.Find(&strm, "? IN STRING_TO_ARRAY(mirrors)", url).Error; e != nil {
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
	if err := dbx.Save(p).Error; err != nil {
		return xerrors.Newf("can't save package in db (pkgid %s): %w", p.ID.String(), err)
	}
	if err := dbx.Save(strm).Error; err != nil {
		return xerrors.Newf("can't save stream in db (strmid %s): %w", strm.ID.String(), err)
	}
	return nil
}

func handleNewStrm(dbx *gorm.DB, p *db.Pkg, urls []string) error {
	l.Debug("new strm", zap.String("ID", p.ID.String()))
	for _, hdlr := range newStrmHdlrs {
		for _, url := range urls {
			if hdlr(dbx, p, url, urls) {
				return nil
			}
		}
	}
	l.Debug("didn't find any existing strm, and no strm supported", zap.String("pkgid", p.ID.String()))
	return dbx.Save(p).Error
}
