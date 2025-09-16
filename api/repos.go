package api

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/terrapkg/gura/db"
	"github.com/terrapkg/gura/util"
)

type RepoPatch struct {
	ID    *string      `json:"id"`
	Type  *db.RepoType `json:"type"`
	Links *string      `json:"links"`
	Fetch *string      `json:"fetch"`
}

// /repos/:repo/ routes
func repoPackageRouteGroup(group *gin.RouterGroup) {
	group.GET("/packages", listPkgsByRepo)

	{
		group.GET("/", fetchRepo)
		group.POST("/", createRepo)
		group.DELETE("/", deleteRepo)
		group.PATCH("/", updateRepo)
	}
}

func listPkgsByRepo(c *gin.Context) {
	repoID := util.SanitizeWhite(c.Param("repo"))

	log.Println("repoID:", repoID) // Debug print
	var pkgs []db.Pkg
	if err := db.DB.Where("repo_id = ?", repoID).Find(&pkgs).Error; err != nil {
		log.Println("listPkgsByRepo: err:", err)
		JSONError(c, 500, err.Error())
		return
	}
	c.JSON(200, pkgs)
}

func deleteRepo(c *gin.Context) {
	repoID := util.SanitizeWhite(c.Param("repo"))
	repo, err := db.RepoFetch(repoID)
	if err != nil {
		log.Println("deleteRepo: err:", err)
		JSONError(c, 500, err.Error())
		return
	}
	if repo == nil {
		JSONError(c, 404, "repo not found")
		return
	}

	// Delete the repo
	// Note: This does not delete associated packages.
	// todo: implement cascading delete in db/repos.go

	if err := repo.Delete(); err != nil {
		log.Println("deleteRepo: delete err:", err)
		JSONError(c, 500, err.Error())
		return
	}
	c.JSON(200, gin.H{"message": "repo deleted"})
}

func fetchRepo(c *gin.Context) {
	repoID := util.SanitizeWhite(c.Param("repo"))
	repo, err := db.RepoFetch(repoID)
	if err != nil {
		log.Println("fetchRepo: err:", err)
		JSONError(c, 500, err.Error())
		return
	}
	if repo == nil {
		JSONError(c, 404, "repo not found")
		return
	}
	c.JSON(200, repo)
}

// Create a new repo if doesn't exist yet
func createRepo(c *gin.Context) {
	repoID := util.SanitizeWhite(c.Param("repo"))
	repoType := util.SanitizeWhite(c.Query("type"))
	if repoType == "" {
		// Default to rpm for now
		repoType = "rpm"
	}

	if repoType != "rpm" {
		JSONError(c, 400, "only 'rpm' repo type is supported at the moment")
		return
	}

	// Check if repo already exists
	existingRepo, err := db.RepoFetch(repoID)
	if err != nil {
		// Real DB error
		log.Println("createRepo: err:", err)
		JSONError(c, 500, err.Error())
		return
	}
	if existingRepo != nil {
		// Repo exists
		JSONError(c, 409, fmt.Sprintf("repo `%s` already exists", repoID))
		return
	}

	// Repo does not exist, create it
	log.Printf("Creating new repo: ID=%s, Type=%s\n", repoID, repoType)

	// convert repoType string to db.RepoType
	// Currently only "rpm" is supported
	var rType db.RepoType
	switch repoType {
	case "rpm":
		rType = db.Rpm
	default:
		JSONError(c, 400, "unsupported repo type")
		return
	}
	res, err := db.RepoCreate(repoID, rType)
	if err != nil {
		log.Printf("createRepo: %v", err)
		JSONError(c, 500, err.Error())
		return
	}

	c.JSON(201, res)
}

func updateRepo(c *gin.Context) {
	repoID := util.SanitizeWhite(c.Param("repo"))

	// Fetch existing repo first
	existingRepo, err := db.RepoFetch(repoID)
	if err != nil {
		JSONError(c, 500, err.Error())
		return
	}
	if existingRepo == nil {
		JSONError(c, 404, "repo not found")
		return
	}

	// Patch structure with pointer fields for partial updates.
	// ID and Type are treated as immutable (cannot change).
	var patch RepoPatch
	if err := c.ShouldBindJSON(&patch); err != nil {
		JSONError(c, 400, "invalid JSON")
		return
	}

	// Reject empty body (no recognized fields)
	if patch.ID == nil && patch.Type == nil && patch.Links == nil && patch.Fetch == nil {
		JSONError(c, 400, "no updatable fields provided")
		return
	}

	// Enforce immutability
	if patch.ID != nil && *patch.ID != repoID {
		JSONError(c, 400, "Repo ID is immutable")
		return
	}
	if patch.Type != nil && *patch.Type != existingRepo.Type {
		JSONError(c, 400, "Repo type is immutable")
		return
	}

	changed := false

	if patch.Links != nil {
		if *patch.Links == "" {
			JSONError(c, 400, "Links cannot be empty")
			return
		}
		existingRepo.Links = *patch.Links
		changed = true
	}
	if patch.Fetch != nil {
		if *patch.Fetch == "" {
			JSONError(c, 400, "Fetch cannot be empty")
			return
		}
		existingRepo.Fetch = *patch.Fetch
		changed = true
	}

	if !changed {
		JSONError(c, 400, "no changes detected")
		return
	}

	if err := existingRepo.Update(); err != nil {
		JSONError(c, 500, err.Error())
		return
	}

	c.JSON(200, existingRepo)
}
