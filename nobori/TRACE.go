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
	close(url_ch)
}

func rpmTrace(p db.Pkg, url_ch chan string) {
	bs, err := p.Meta.MarshalJSON()
	if util.Yeet(l, "error marshaling package metadata", err, zap.String("id", p.ID.String())) {
		l.DPanic("DPanic")
		return
	}

	var meta repomd.RPMMeta
	util.MaybeSuicide(l, "cannot unmarshal RPMMeta", json.Unmarshal(bs, &meta))
	url_ch <- meta.Url
}
