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
func (r Repo) Delete() error {
	if err := DB.Delete(&r).Error; err != nil {
		return err
	}
	return nil
}
