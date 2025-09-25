/*
gura

Copyright (c) 2024-2025 Fyra Labs

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package db

import (
	"os"
	"strings"
	"time"

	"github.com/terrapkg/gura/util"
	dt "gorm.io/datatypes"
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
	ID      dt.UUID   `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	LastChk time.Time `json:"last_chk"`
	LastUpd time.Time `json:"last_upd"`
	Fetch   string    `json:"fetch"`
	Forge   ForgeType `json:"forge"`
	Ver     string    `json:"ver"`

	Mirrors []StreamMirror `json:"mirrors"` // comma-separated list of mirrors
}

func (s *Stream) BeforeCreate(tx *gorm.DB) (err error) {
	if s.ID.IsEmpty() {
		s.ID = dt.NewUUIDv4()
	}
	return nil
}

type StreamMirror struct {
	ID       dt.UUID `gorm:"type:uuid" json:"id"`
	StreamID dt.UUID `gorm:"type:uuid" json:"streamid"`
	Stream   Stream  `json:"-"`
	Mirror   string  `json:"mirror"` // url without `http(s)://` and trailing slash
}

func (m *StreamMirror) BeforeCreate(tx *gorm.DB) error {
	if m.ID.IsEmpty() {
		m.ID = dt.NewUUIDv4()
	}
	return nil
}
func (m *StreamMirror) BeforeSave(tx *gorm.DB) error {
	m.Mirror = strings.TrimSuffix(m.Mirror, "/")
	return nil
}

// Pkg is the package model. Uses UUID primary key instead of gorm.Model's uint.
type Pkg struct {
	ID        dt.UUID        `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
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

	Meta dt.JSON `gorm:"type:jsonb;default:'{}'" json:"meta"`

	StreamID *dt.UUID `gorm:"type:uuid" json:"stream_id"`
	Stream   Stream   `json:"-"`
}

func (s *Pkg) BeforeCreate(tx *gorm.DB) (err error) {
	if s.ID.IsEmpty() {
		s.ID = dt.NewUUIDv4()
	}
	return nil
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
	l.Info("setting up db")
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
	util.MaybeSuicide(l, "auto migrate failed", DB.AutoMigrate(&Repo{}, &Pkg{}, &Stream{}, &StreamMirror{}))
	l.Info("db ready")
}
