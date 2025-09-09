package db

import (
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
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
	ID    uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();not null;primaryKey" json:"id"`
	PkgID uuid.UUID      `gorm:"not null;index;constraint:OnDelete:CASCADE" json:"pkg_id"`
	Key   string         `gorm:"not null;index" json:"key"`
	Data  datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"data"`
}

// Pkg is the package model. Uses UUID primary key instead of gorm.Model's uint.
type Pkg struct {
	ID        datatypes.UUID `gorm:"type:uuid;default:gen_random_uuid();not null;primaryKey" json:"id"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	Name    string `json:"name"`
	FullVer string `json:"full_ver"`
	Ver     string `json:"ver"`
	Arch    string `json:"arch"`
	// RepoID is a string (not a UUID). Repositories are identified by string IDs.
	RepoID string `gorm:"not null;index" json:"repo_id"`
	// Foreign key relationship referencing Repo.ID (string).
	Repo Repo `gorm:"constraint:OnDelete:CASCADE;foreignKey:RepoID;references:ID" json:"-"`

	Metas []PkgMeta
}

// Repo represents a package repository. Its ID is a string.
type Repo struct {
	ID    string    `gorm:"primaryKey" json:"id"`
	UpdAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
	Links string    `json:"links"`
	Type  RepoType  `json:"type"`
	Fetch string    `json:"fetch"`
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
