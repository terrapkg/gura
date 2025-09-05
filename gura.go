package gura

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var ver = "version is not set!"

func health(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, ver)
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalln("cannot load .env", err)
	}

	db, err := gorm.Open(postgres.Open(os.Getenv("GURA_DSN")), &gorm.Config{})
	if err != nil {
		log.Fatalln("cannot open db: ", err)
	}
	ctx := context.Background()

	// Migrate the schema
	db.AutoMigrate(&Product{})

	http.HandleFunc("/health", health)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
