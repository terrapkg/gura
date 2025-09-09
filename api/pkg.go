package api

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/terrapkg/gura/db"
)

func onePackageRouteGroup(group *gin.RouterGroup) {
	group.GET("/", getPackage)
	group.DELETE("/", deletePackage)
	group.PATCH("/", updatePackage)
	group.POST("/", submitPackage)
}

func packageRouteGroup(group *gin.RouterGroup) {
	onePackageRouteGroup(group.Group("/:id"))
}

// Listing all packages is intentionally unimplemented, there might be millions of packages.

func getPackage(c *gin.Context) {
	id := c.Param("id")
	pkg, err := db.PkgFetch(uuid.MustParse(id))
	if err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, pkg)
}

func deletePackage(c *gin.Context) {
	id := c.Param("id")
	pkg, err := db.PkgFetch(uuid.MustParse(id))
	if err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}
	if pkg == nil {
		c.JSON(404, gin.H{"error": "package not found"})
		return
	}
	if err := pkg.Delete(); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "package deleted"})
}

func updatePackage(c *gin.Context) {
	id := c.Param("id")
	pkg, err := db.PkgFetch(uuid.MustParse(id))
	if err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}
	if pkg == nil {
		c.JSON(404, gin.H{"error": "package not found"})
		return
	}
	if err := c.BindJSON(pkg); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if err := pkg.Update(); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, pkg)
}

func submitPackage(c *gin.Context) {
	var pkg db.Pkg
	if err := c.BindJSON(&pkg); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	submittedPkg, err := db.SubmitPkg(&pkg)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, submittedPkg)
}
