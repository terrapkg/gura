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

	"gorm.io/gorm"
)

func RepoFetch(repo_id string) (*Repo, error) {
	var repo Repo
	if err := DB.Where("id = ?", repo_id).First(&repo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Not found -> return nil result and no error so callers can distinguish
			return nil, nil
		}
		// Some other DB error
		return nil, err
	}
	return &repo, nil
}

func RepoFetchAll() []Repo {
	var repos []Repo
	DB.Find(&repos)
	return repos
}

func RepoCreate(repo_id string, repo_type RepoType) (*Repo, error) {
	repo := Repo{
		ID:   repo_id,
		Type: repo_type,
	}
	if err := DB.Create(&repo).Error; err != nil {
		return nil, err
	}
	return &repo, nil
}

// method to delete a repo
// Repo.Delete()
func (r *Repo) Delete() error {
	return DB.Delete(r).Error
}

func (r *Repo) Update() error {
	return DB.Save(r).Error
}
