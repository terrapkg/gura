package db

import (
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func PkgFetch(pkg_id uuid.UUID) (*Pkg, error) {
	var pkg Pkg
	if err := DB.Where("id = ?", pkg_id).First(&pkg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Not found -> return nil result and no error so callers can distinguish
			return nil, nil
		}
		// Some other DB error
		return nil, err
	}
	return &pkg, nil
}

func PkgFetchMulti(pkg_ids []uuid.UUID) ([]Pkg, error) {
	var pkgs []Pkg
	if err := DB.Where("id IN ?", pkg_ids).Find(&pkgs).Error; err != nil {
		return nil, err
	}
	return pkgs, nil
}

// GORM helper for querying packages with arbitrary conditions
func PkgQuerySQL(query interface{}, args ...interface{}) ([]Pkg, error) {
	var pkgs []Pkg
	if err := DB.Where(query, args...).Find(&pkgs).Error; err != nil {
		return nil, err
	}
	return pkgs, nil
}

func SubmitPkg(pkg *Pkg) (*Pkg, error) {
	if err := DB.Create(pkg).Error; err != nil {
		return nil, err
	}
	return pkg, nil
}

func (p *Pkg) Update() error {
	if err := DB.Save(p).Error; err != nil {
		return err
	}
	return nil
}

func (p *Pkg) Delete() error {
	if err := DB.Delete(p).Error; err != nil {
		return err
	}
	return nil
}
