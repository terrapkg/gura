package api

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/terrapkg/gura/db"
)

type PkgPatch struct {
	Name    *string `json:"name"`
	FullVer *string `json:"full_ver"`
	Ver     *string `json:"ver"`
	Arch    *string `json:"arch"`
	RepoID  *string `json:"repo_id"`
}

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
	pkgID, err := uuid.Parse(id)
	if err != nil {
		JSONError(c, 400, "invalid UUID")
		return
	}
	pkg, err := db.PkgFetch(pkgID)
	c.JSON(200, pkg)
}

func deletePackage(c *gin.Context) {
	id := c.Param("id")
	pkg, err := db.PkgFetch(uuid.MustParse(id))
	if err != nil {
		JSONError(c, 404, err.Error())
		return
	}
	if pkg == nil {
		JSONError(c, 404, "package not found")
		return
	}
	if err := pkg.Delete(); err != nil {
		JSONError(c, 500, err.Error())
		return
	}
	c.JSON(200, gin.H{"message": "package deleted"})
}

func updatePackage(c *gin.Context) {
	id := c.Param("id")
	pkg, err := db.PkgFetch(uuid.MustParse(id))
	if err != nil {
		JSONError(c, 404, err.Error())
		return
	}
	if pkg == nil {
		JSONError(c, 404, "package not found")
		return
	}
	var patch PkgPatch
	if err := c.ShouldBindJSON(&patch); err != nil {
		JSONError(c, 400, err.Error())
		return
	}

	// Reject empty body (no updatable fields provided)
	if patch.Name == nil && patch.FullVer == nil && patch.Ver == nil && patch.Arch == nil && patch.RepoID == nil {
		JSONError(c, 400, "no updatable fields provided")
		return
	}

	// RepoID is immutable; if provided and different, reject.
	if patch.RepoID != nil && *patch.RepoID != pkg.RepoID {
		JSONError(c, 400, "RepoID is immutable")
		return
	}

	changed := false

	// Apply only provided (non-nil) fields; immutable fields (ID, timestamps, metas, RepoID) are ignored.
	if patch.Name != nil {
		if *patch.Name == "" {
			JSONError(c, 400, "Name cannot be empty")
			return
		}
		pkg.Name = *patch.Name
		changed = true
	}
	if patch.FullVer != nil {
		pkg.FullVer = *patch.FullVer
		changed = true
	}
	if patch.Ver != nil {
		if *patch.Ver == "" {
			JSONError(c, 400, "Ver cannot be empty")
			return
		}
		pkg.Ver = *patch.Ver
		changed = true
	}
	if patch.Arch != nil {
		if *patch.Arch == "" {
			JSONError(c, 400, "Arch cannot be empty")
			return
		}
		pkg.Arch = *patch.Arch
		changed = true
	}
	// patch.RepoID intentionally not applied (immutable)

	if !changed {
		JSONError(c, 400, "no changes detected")
		return
	}

	if err := pkg.Update(); err != nil {
		JSONError(c, 500, err.Error())
		return
	}
	c.JSON(200, pkg)
}

func submitPackage(c *gin.Context) {
	var patch PkgPatch
	if err := c.ShouldBindJSON(&patch); err != nil {
		JSONError(c, 400, err.Error())
		return
	}

	// Validate required fields for new package submission
	if patch.Name == nil || *patch.Name == "" {
		JSONError(c, 400, "Name is required and cannot be empty")
		return
	}
	if patch.FullVer == nil || *patch.FullVer == "" {
		JSONError(c, 400, "FullVer is required and cannot be empty")
		return
	}
	if patch.Ver == nil || *patch.Ver == "" {
		JSONError(c, 400, "Ver is required and cannot be empty")
		return
	}
	if patch.Arch == nil || *patch.Arch == "" {
		JSONError(c, 400, "Arch is required and cannot be empty")
		return
	}
	if patch.RepoID == nil || *patch.RepoID == "" {
		JSONError(c, 400, "RepoID is required and cannot be empty")
		return
	}

	pkg := db.Pkg{
		Name:    *patch.Name,
		FullVer: *patch.FullVer,
		Ver:     *patch.Ver,
		Arch:    *patch.Arch,
		RepoID:  *patch.RepoID,
	}

	submittedPkg, err := db.SubmitPkg(&pkg)
	if err != nil {
		JSONError(c, 500, err.Error())
		return
	}
	c.JSON(200, submittedPkg)
}
