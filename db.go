package gura

import (
	"log"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type PkgMeta struct {
	ID    uint64
	PkgID uint64
	Key   string
	Val   string
}
type Pkg struct {
	gorm.Model
	Name    string
	FullVer string
	Ver     string // compatible version string between repos
	Arch    string

	RepoID string
	Repo   Repo

	Metas []PkgMeta
}
type Repo struct {
	//gorm.Model
	ID    string
	Links string
	Type  RepoType
}

type RepoType int64

const (
	Rpm RepoType = iota
)

var DB *gorm.DB

func SetupDB() {
	db, err := gorm.Open(postgres.Open(os.Getenv("GURA_DSN")), &gorm.Config{})
	if err != nil {
		log.Fatalln("cannot open db: ", err)
	}

	db.AutoMigrate(&PkgMeta{}, &Pkg{}, &Repo{})
	DB = db
}
