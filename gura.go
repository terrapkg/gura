package gura

import (
	"fmt"
	"log"
	"net/http"

	"github.com/joho/godotenv"
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

	SetupDB()

	http.HandleFunc("/health", health)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
