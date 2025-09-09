package api

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/terrapkg/gura/db"
)

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
	repoID := c.Param("repo")

	fmt.Println("repoID:", repoID) // Debug print
	var pkgs []db.Pkg
	if err := db.DB.Where("repo_id = ?", repoID).Find(&pkgs).Error; err != nil {
		log.Println("listPkgsByRepo: err:", err)
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, pkgs)
}

func deleteRepo(c *gin.Context) {
	repoID := c.Param("repo")
	repo, err := db.RepoFetch(repoID)
	if err != nil {
		log.Println("deleteRepo: err:", err)
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if repo == nil {
		c.JSON(404, gin.H{"error": "repo not found"})
		return
	}

	// Delete the repo
	// Note: This does not delete associated packages.
	// todo: implement cascading delete in db/repos.go

	if err := repo.Delete(); err != nil {
		log.Println("deleteRepo: delete err:", err)
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "repo deleted"})
}

func fetchRepo(c *gin.Context) {
	repoID := c.Param("repo")
	repo, err := db.RepoFetch(repoID)
	if err != nil {
		log.Println("fetchRepo: err:", err)
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if repo == nil {
		c.JSON(404, gin.H{"error": "repo not found"})
		return
	}
	c.JSON(200, repo)
}

// Create a new repo if doesn't exist yet
func createRepo(c *gin.Context) {
	repoID := c.Param("repo")
	repoType := c.Query("type")
	if repoType == "" {
		// Default to rpm for now
		repoType = "rpm"
	}

	if repoType != "rpm" {
		c.JSON(400, gin.H{"error": "only 'rpm' repo type is supported at the moment"})
		return
	}

	// Check if repo already exists
	existingRepo, err := db.RepoFetch(repoID)
	if err != nil {
		// Real DB error
		log.Println("createRepo: err:", err)
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if existingRepo != nil {
		// Repo exists
		c.JSON(409, gin.H{"error": fmt.Sprintf("repo `%s` already exists", repoID)})
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
		c.JSON(400, gin.H{"error": "unsupported repo type"})
		return
	}
	res, err := db.RepoCreate(repoID, rType)
	if err != nil {
		log.Printf("createRepo: %v", err)
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(201, res)
}

func updateRepo(c *gin.Context) {
	repo_id := c.Param("repo")

	var repo db.Repo

	if err := c.ShouldBindJSON(&repo); err != nil {
		c.JSON(400, gin.H{"error": "Invalid JSON"})
		return
	}

	if repo.ID != repo_id {
		c.JSON(400, gin.H{"error": "Repo ID in URL and body do not match"})
		return
	}

	existingRepo, err := db.RepoFetch(repo_id)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if existingRepo == nil {
		c.JSON(404, gin.H{"error": "Repo not found"})
		return
	}

	if err := repo.Update(); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, repo)
}
