package main

import (
	"crash-game/internal/database"
	"crash-game/internal/server"
	"log"
)

func main() {
	db, err := database.NewDatabase("postgres://crashgamedb_user:March0s0ft@localhost:5432/crashgame?sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}

	gameServer := server.NewGameServer(db)
	if err := gameServer.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
