// Determine stream of a package.
package db

import (
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"strings"

	"github.com/mdobak/go-xerrors"
	"github.com/terrapkg/gura/repomd"
	"github.com/terrapkg/gura/util"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func RegPkg(p Pkg) error {
	if p.StreamID != nil {
		return DB.Save(p).Error
	}
	url_ch := make(chan string)
	UpTrace(p, url_ch)
	close(url_ch)
	urls := map[string]struct{}{}
	var strm *Stream
recv:
	url, ok := <-url_ch
	if !ok {
		// TODO: logic for handling new stream
		goto done
	}
	if e := DB.Find(&strm, "? IN STRING_TO_ARRAY(mirrors)", url).Error; e != nil {
		if errors.Is(e, gorm.ErrRecordNotFound) {
			goto recv
		}
		return xerrors.Newf("error finding stream in db (pkgid %s): %w", p.ID.String(), e)
	}
done:
	for url := range strings.SplitSeq(strm.Mirrors, ",") {
		urls[url] = struct{}{}
	}
	for url, ok := <-url_ch; ok; {
		urls[url] = struct{}{}
	}

	strm.Mirrors = strings.Join(slices.Collect(maps.Keys(urls)), ",")
	p.StreamID = &strm.ID
	if err := DB.Save(p).Error; err != nil {
		return err
	}
	if err := DB.Save(strm).Error; err != nil {
		return err
	}
	return nil
}

func UpTrace(p Pkg, url_ch chan string) {
	switch p.Repo.Type {
	case Rpm:
		rpmTrace(p, url_ch)
	default:
		l.DPanic("unreachable in uptrace")
	}
}

func rpmTrace(p Pkg, url_ch chan string) {
	bs, err := p.Meta.MarshalJSON()
	if util.Yeet(l, "error marshaling package metadata", err, zap.String("id", p.ID.String())) {
		l.DPanic("DPanic")
		return
	}

	var meta repomd.RPMMeta
	json.Unmarshal(bs, &meta)
	url_ch <- meta.Url
}
