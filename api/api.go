// API router
//
// This module acts as an API router for the application.
//

package api

import (
	"time"

	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"github.com/terrapkg/gura/util"
)

var log = util.SetupLog("api")

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
	log.Info("Initializing API service")

	router := gin.Default()
	router.Use(ginzap.Ginzap(log, time.RFC3339, true))
	router.Use(ginzap.RecoveryWithZap(log, true))

	router.GET("/health", getHealth(opts.Version))

	packageRouteGroup(router.Group("/packages"))
	repoPackageRouteGroup(router.Group("/repos/:repo"))

	return router
}
