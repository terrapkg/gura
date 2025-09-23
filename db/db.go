package db

import (
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/terrapkg/gura/util"
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"moul.io/zapgorm2"
)

// RepoType represents repository type enum.
type RepoType int64

const (
	Rpm RepoType = iota
)

// ForgeType represents forge type enum.
type ForgeType int64

const (
	GitHub ForgeType = iota
)

// Streams are grouped packages with the same upstream
type Stream struct {
	ID      uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	LastChk time.Time `json:"last_chk"`
	LastUpd time.Time `json:"last_upd"`
	Fetch   string    `json:"fetch"`
	Forge   ForgeType `json:"forge"`
	Ver     string    `json:"ver"`
	Mirrors string    `json:"mirrors"` // comma-separated list of mirrors
}

// Pkg is the package model. Uses UUID primary key instead of gorm.Model's uint.
type Pkg struct {
	ID        datatypes.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at"`

	Name    string `json:"name"`
	FullVer string `json:"full_ver"`
	Ver     string `json:"ver"`
	Arch    string `json:"arch"`
	// RepoID is a string (not a UUID). Repositories are identified by string IDs.
	RepoID string `gorm:"not null;index" json:"repo_id"`
	// Foreign key relationship referencing Repo.ID (string).
	Repo Repo `gorm:"constraint:OnDelete:CASCADE;foreignKey:RepoID;references:ID" json:"-"`

	Meta datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"meta"`

	StreamID *uuid.UUID `json:"stream_id"`
	Stream   Stream     `json:"-"`
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
var l = util.SetupLog("db")

// SetupDB initializes the global DB connection and runs migrations.
func SetupDB() {
	var err error
	dsn := os.Getenv("GURA_DSN")
	if dsn == "" {
		l.Fatal("GURA_DSN environment variable is not set")
	}

	logger := zapgorm2.New(util.SetupLog("gorm"))
	logger.SetAsDefault() // optional: configure gorm to use this zapgorm.Logger for callbacks
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger})
	util.MaybeSuicide(l, "cannot open db", err)

	// AutoMigrate in an order that respects foreign keys.
	util.MaybeSuicide(l, "auto migrate failed", DB.AutoMigrate(&Repo{}, &Pkg{}, &Stream{}))
}
