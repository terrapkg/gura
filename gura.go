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

package main

import (
	"os"

	"github.com/terrapkg/gura/api"
	"github.com/terrapkg/gura/db"
	"github.com/terrapkg/gura/kudari"
	"github.com/terrapkg/gura/nobori"
	"github.com/terrapkg/gura/util"
	"go.uber.org/zap"
)

// Set this variable by adding the build flag: -ldflags '-X main.ver=1.2.3'
var ver = "version is not set!"
var l = util.SetupLog("gura")

func main() {
	zap.ReplaceGlobals(l)
	// godotenv is already loaded by util.SetupLog

	router_opts := api.RouterSetupOpts{
		Version: ver,
	}

	db.SetupDB()
	nobori.StartFetchLoop()
	go kudari.FetchLoop()

	router := api.SetupRouter(router_opts)
	var listen_address = ":8080"
	laddress := os.Getenv("GURA_LISTEN_ADDRESS")
	if laddress != "" {
		listen_address = laddress
	}

	router.Run(listen_address)
}
