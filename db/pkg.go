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

func SubmitPkg(pkg *Pkg) (*Pkg, error) {
	// Don't actually take the package ID when submitting, we'll generate a new one.
	pkg.ID = uuid.Nil
	pkg.CreatedAt = DB.NowFunc()
	pkg.UpdatedAt = DB.NowFunc()
	if err := DB.Create(pkg).Error; err != nil {
		return nil, err
	}
	return pkg, nil
}

func (p *Pkg) Update() error {
	p.UpdatedAt = DB.NowFunc()
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
