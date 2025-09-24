// nobori: upstream metadata fetching from various streams
//
// `TRACE.go`: Obtain traces from packages.
// Traces are currently just URLs that potentially point to upstream.
//
// Named after #TRACE_pr.

package nobori

import (
	"encoding/json"

	"github.com/terrapkg/gura/db"
	"github.com/terrapkg/gura/repomd"
	"github.com/terrapkg/gura/util"
	"go.uber.org/zap"
)

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
