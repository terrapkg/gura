package gura

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/terrapkg/gura/db"
	"github.com/terrapkg/gura/kudari"
	"github.com/terrapkg/gura/nobori"
)

var ver = "version is not set!"

func getHealth(c *gin.Context) {
	c.String(200, ver)
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalln("cannot load .env", err)
	}

	db.SetupDB()
	go kudari.FetchLoop()
	go nobori.FetchLoop()

	router := gin.Default()
	router.GET("/health", getHealth)
	router.Run("localhost:8080")
}
