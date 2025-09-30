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

package nobori

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTokRotations(t *testing.T) {
	ghTokmgr.rtIdx = 0
	ghTokmgr.rtToks = []GHToken{
		{key: "0"},
		{key: "1", quota: 1},
		{key: "2"},
	}
	ghTokmgr.qlIdx = 0
	ghTokmgr.qlToks = []GHToken{
		{key: "0"},
		{key: "1", quota: 1},
		{key: "2"},
	}

	rt, ql := ghTokmgr.rt(), ghTokmgr.ql()
	assert.Equal(t, rt.key, "0", "first key")
	assert.Equal(t, ql.key, "0", "first key")

	ghTokmgr.thanksForAllTheFishRt(&rt)
	assert.Equal(t, rt, &ghTokmgr.rtToks[1], "key rotated")
	ghTokmgr.thanksForAllTheFishQl(&ql)
	assert.Equal(t, ql, &ghTokmgr.qlToks[1], "key rotated")

	assert.False(t, rt.noMoreFish(), "should have fish")
	assert.False(t, ql.noMoreFish(), "should have fish")

	ghTokmgr.rtToks[1].quota = 0
	ghTokmgr.qlToks[1].quota = 0

	ghTokmgr.thanksForAllTheFishRt(&rt)
	ghTokmgr.thanksForAllTheFishRt(&rt)
	ghTokmgr.thanksForAllTheFishQl(&ql)
	ghTokmgr.thanksForAllTheFishQl(&ql)

	assert.Equal(t, rt, &ghTokmgr.rtToks[0], "key rotated")
	assert.Equal(t, ql, &ghTokmgr.qlToks[0], "key rotated")
}
