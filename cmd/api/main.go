// Command api serves the pickup-session API.
// PICKUP_DSN set → Postgres store, otherwise in-memory. PORT defaults to 8080.
package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/Soufiane1412/pickup-session/api"
	"github.com/Soufiane1412/pickup-session/session"
)

func main() {
	var store api.Store = session.NewMemoryStore()
	if dsn := os.Getenv("PICKUP_DSN"); dsn != "" {
		pg, err := session.NewPostgresStore(context.Background(), dsn)
		if err != nil {
			log.Fatal(err)
		}
		store = pg
		log.Println("store: postgres")
	} else {
		log.Println("store: memory")
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Println("listening on :" + port)
	log.Fatal(http.ListenAndServe(":"+port, api.New(store)))
}
