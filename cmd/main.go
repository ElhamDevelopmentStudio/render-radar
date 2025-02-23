package main

import (
	"debugger-api/internal/server"
	"log"
)

func main() {
	if err := server.SetupAndRun(); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
} 