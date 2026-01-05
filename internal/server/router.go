package server

import (
	"github.com/gofiber/fiber/v2"
)

// SetupRoutes sets up the API router with versioning and route groups
func SetupRoutes(app *fiber.App) {
	// Version 1 API group
	v1 := app.Group("/api/v1")

	// Stack management endpoints
	stacks := v1.Group("/stacks")
	_ = stacks // TODO: Register stack handlers here

	// Deployment management endpoints
	deployments := v1.Group("/deployments")
	_ = deployments // TODO: Register deployment handlers here

	// Webhook handling endpoints
	webhooks := v1.Group("/webhooks")
	_ = webhooks // TODO: Register webhook handlers here

	// Health check endpoints
	health := v1.Group("/health")
	_ = health // TODO: Register health handlers here, or use existing health handler
}
