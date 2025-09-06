package main

import (
	"log"

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
	log.Println("Starting server on localhost:8080")
	router.Run(":8080")
}
