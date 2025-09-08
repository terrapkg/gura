package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/terrapkg/gura/api"
	"github.com/terrapkg/gura/db"
	"github.com/terrapkg/gura/kudari"
	"github.com/terrapkg/gura/nobori"
)

var ver = "version is not set!"

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalln("cannot load .env", err)
	}

	router_opts := api.RouterSetupOpts{
		Version: ver,
	}

	db.SetupDB()
	go kudari.FetchLoop()
	go nobori.FetchLoop()

	router := api.SetupRouter(router_opts)
	var listen_address = ":8080"
	laddress := os.Getenv("GURA_LISTEN_ADDRESS")
	if laddress != "" {
		listen_address = laddress
	}

	router.Run(listen_address)
}
