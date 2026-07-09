package web

import (
	"github.com/gin-gonic/gin"
)

type server struct {
	env string
}

type ServerConfig struct {
	Env string
}

func NewServer(config ServerConfig) *server {
	return &server{env: config.Env}
}

func (s *server) GinEngine() *gin.Engine {
	router := s.newGinEngine()

	router.GET("/health_check", s.healthCheck)
	router.POST("/api/email/inbound", s.handleInbound)

	return router
}

func (s *server) newGinEngine() *gin.Engine {
	if s.env == "production" {
		gin.SetMode(gin.ReleaseMode)
		router := gin.New()
		router.Use(gin.Recovery())
		return router
	}

	gin.SetMode(gin.DebugMode)
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	return router
}

func (s *server) healthCheck(c *gin.Context) {
	c.JSON(200, gin.H{
		"service": "sr-email",
		"status":  "ok",
	})
}