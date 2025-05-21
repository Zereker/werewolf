package main

import (
	"log"
	"os"

	"github.com/Zereker/werewolf/pkg/server"
	"github.com/Zereker/werewolf/pkg/werewolf"
)

func main() {
	// Create a new game runtime
	rt := werewolf.NewRuntime()

	// Create a new game server, passing the runtime instance
	srv, err := server.NewServer(":8080", rt)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
		os.Exit(1)
	}

	// Log initialization
	log.Println("Server and runtime initialized successfully.")

	// Start the server
	log.Println("Starting server on :8080")
	if err := srv.Serve(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
		os.Exit(1)
	}
}
