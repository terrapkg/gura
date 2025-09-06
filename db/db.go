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
	ID    uuid.UUID `gorm:"type:uuid;primaryKey"`
	PkgID uuid.UUID `gorm:"type:uuid;index"`
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
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey"`
	CreatedAt time.Time      `gorm:"autoCreateTime"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"index"`

	Name    string
	FullVer string
	// compatible version string between repos
	Ver  string
	Arch string

	// RepoID is a string (not a UUID). Repositories are identified by string IDs.
	RepoID string `gorm:"index;type:text"`
	// Foreign key relationship referencing Repo.ID (string).
	Repo Repo `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;foreignKey:RepoID;references:ID"`

	Metas []PkgMeta `gorm:"foreignKey:PkgID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

// BeforeCreate sets a UUID for Pkg if not provided.
func (p *Pkg) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

func ListAllPkgs() ([]Pkg, error) {
	var pkgs []Pkg
	if err := DB.Find(&pkgs).Error; err != nil {
		return nil, err
	}
	return pkgs, nil
}

func GetPackageByID(id uuid.UUID) (*Pkg, error) {
	var pkg Pkg
	if err := DB.Where("id = ?", id).First(&pkg).Error; err != nil {
		return nil, err
	}
	return &pkg, nil
}

// Repo represents a package repository. Its ID is a string.
type Repo struct {
	ID    string    `gorm:"primaryKey;type:text"`
	UpdAt time.Time `gorm:"autoUpdateTime"`
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
