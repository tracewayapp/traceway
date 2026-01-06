package main

import (
	"backend/app/cache"
	"backend/app/chdb"
	"backend/app/controllers"
	"backend/app/middleware"
	"backend/app/migrations"
	"context"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	err = chdb.Init()
	if err != nil {
		panic(err)
	}

	err = migrations.Run()
	if err != nil {
		panic(err)
	}

	// Initialize project cache
	ctx := context.Background()
	if err := cache.ProjectCache.Init(ctx); err != nil {
		panic(err)
	}

	middleware.InitUseClientAuth()

	router := gin.Default()

	router.Use(gin.Recovery())

	apiRouterGroup := router.Group("/api")
	controllers.RegisterControllers(apiRouterGroup)

	if err := router.Run(":8082"); err != nil {
		panic(err)
	}
}
