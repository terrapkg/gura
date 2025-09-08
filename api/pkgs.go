package api

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/terrapkg/gura/db"
)

func packageRouteGroup(group *gin.RouterGroup) {
	// group.GET("/", func(c *gin.Context) {
	// 	pkgs, err := ListAllPkgs()
	// 	if err != nil {
	// 		c.JSON(500, gin.H{"error": err.Error()})
	// 		return
	// 	}
	// 	c.JSON(200, pkgs)
	// })
	group.GET("/:id", getPackage)
}

// func ListAllPkgs() ([]db.Pkg, error) {
// 	var pkgs []db.Pkg
// 	if err := db.DB.Find(&pkgs).Error; err != nil {
// 		return nil, err
// 	}
// 	return pkgs, nil
// }

func GetPackageByID(id uuid.UUID) (*db.Pkg, error) {
	var pkg db.Pkg
	if err := db.DB.Where("id = ?", id).First(&pkg).Error; err != nil {
		return nil, err
	}
	return &pkg, nil
}
