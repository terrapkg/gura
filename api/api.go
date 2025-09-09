// API router
//
// This module acts as an API router for the application.
//

package api

import (
	"log"

	"github.com/gin-gonic/gin"
)

func JSONError(c *gin.Context, code int, msg string) {
    c.JSON(code, gin.H{"error": msg})
}


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
