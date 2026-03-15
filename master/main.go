package main

import (
	"log"
	"math/rand"
	"net/http"
	"time"

	"gpu-optimizer/master/handlers"
	"gpu-optimizer/master/worker"

	"github.com/gin-gonic/gin"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	manager := worker.NewManager()
	wsHub := handlers.NewWebSocketHub(manager)

	go wsHub.Run()
	go wsHub.StartBroadcasting()

	router := gin.Default()

	// CORS
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	apiHandler := handlers.NewAPIHandler(manager, wsHub)
	apiHandler.RegisterRoutes(router)

	// WebSocket endpoint
	router.GET("/ws", wsHub.HandleWebSocket)

	// Статика для фронтенда (в продакшене лучше через nginx)
	router.Static("/static", "./static")

	log.Println("🚀 Мастер запущен на :8080")
	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatal(err)
	}
}
