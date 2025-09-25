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
