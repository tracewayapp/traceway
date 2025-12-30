package main

import (
	"backend/app/chdb"
	"backend/app/controllers"
	"backend/app/middleware"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	chdb.Init()

	middleware.InitUseClientAuth()

	router := gin.Default()

	router.Use(gin.Recovery())

	apiRouterGroup := router.Group("/api")
	controllers.RegisterControllers(apiRouterGroup)

	if err := router.Run(":8082"); err != nil {
		panic(err)
	}
}
