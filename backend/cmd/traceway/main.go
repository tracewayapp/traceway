package main

import tracewaybackend "github.com/tracewayapp/traceway/backend"

func main() {
	tracewaybackend.Run()
	// tracewaybackend.Run(
	// 	tracewaybackend.WithPort(8082),
	// 	tracewaybackend.WithDefaultUser("admin@localhost.com", "admin"),
	// 	tracewaybackend.WithDefaultProject("Backend", "gin", "backend-dev-token"),
	// 	tracewaybackend.WithDefaultProject("Frontend", "svelte", "frontend-dev-token"),
	// )
}
