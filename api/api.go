// API router
//
// This module acts as an API router for the application.
//

package api

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Holds options for routing settings,
// right now only contains version string,
// but will be expanded in the future.
type RouterSetupOpts struct {
	Version string
}

func getHealth(version string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.String(200, version)
	}
}

func SetupRouter(opts RouterSetupOpts) *gin.Engine {
	log.Println("Initializing API service")

	router := gin.Default()

	router.GET("/health", getHealth(opts.Version))

	packageRouteGroup(router.Group("/packages"))
	repoPackageRouteGroup(router.Group("/repos/:repo"))

	return router
}

func getPackage(c *gin.Context) {
	id := c.Param("id")
	pkg, err := GetPackageByID(uuid.MustParse(id))
	if err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, pkg)
}

