package db

import (
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// RepoType represents repository type enum.
type RepoType int64

const (
	Rpm RepoType = iota
)

// PkgMeta stores arbitrary metadata for a package.
type PkgMeta struct {
	ID    uuid.UUID `gorm:"primaryKey"`
	PkgID uuid.UUID `gorm:"index"`
	Key   string
	Val   string
}

// BeforeCreate ensures a UUID is set before inserting into DB.
func (m *PkgMeta) BeforeCreate(tx *gorm.DB) (err error) {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

// Pkg is the package model. Uses UUID primary key instead of gorm.Model's uint.
type Pkg struct {
	ID        uuid.UUID `gorm:"primaryKey,unique"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	Name    string
	FullVer string
	Ver     string // compatible version string between repos
	Arch    string

	RepoID string // RepoID is a string (not a UUID). Repositories are identified by string IDs.
	Repo Repo // Foreign key relationship referencing Repo.ID (string).

	Metas []PkgMeta
}

// BeforeCreate sets a UUID for Pkg if not provided.
func (p *Pkg) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// Repo represents a package repository. Its ID is a string.
type Repo struct {
	ID    string    `gorm:"primaryKey"`
	UpdAt time.Time //`gorm:"autoUpdateTime"`
	Links string
	Type  RepoType
	Fetch string
}

var DB *gorm.DB

// SetupDB initializes the global DB connection and runs migrations.
func SetupDB() {
	var err error
	dsn := os.Getenv("GURA_DSN")
	if dsn == "" {
		log.Fatalln("GURA_DSN environment variable is not set")
	}

	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalln("cannot open db: ", err)
	}

	// AutoMigrate in an order that respects foreign keys.
	if err := DB.AutoMigrate(&Repo{}, &Pkg{}, &PkgMeta{}); err != nil {
		log.Fatalln("auto migrate failed: ", err)
	}
}
