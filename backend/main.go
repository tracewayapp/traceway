package main

import (
	"backend/app/controllers"
	"backend/app/middleware"
	"log"

	"github.com/gin-gonic/gin"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// db.Init()
	// datawarehouse.Init()

	// migrations.Run()

	middleware.InitUseAuth()

	router := gin.Default()

	router.Use(gin.Recovery())

	apiRouterGroup := router.Group("/api")
	controllers.RegisterControllers(apiRouterGroup)

	if err := router.Run(":8082"); err != nil {
		panic(err)
	}
}
