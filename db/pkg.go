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

func (s *Pkg) Update() error {
	if err := DB.Save(s).Error; err != nil {
		return err
	}
	return nil
}

func (s *Pkg) Delete() error {
	if err := DB.Delete(s).Error; err != nil {
		return err
	}
	return nil
}
