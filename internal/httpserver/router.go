package httpserver

import "github.com/gin-gonic/gin"

func NewRouter() *gin.Engine {

	router := gin.New()

	router.Use(gin.Logger())

	router.Use(gin.Recovery())

	router.GET("/health", health)

	return router
}
