package main

import (
	"log"
	"net/http"
	"os"

	"github.com/p3ym4n/concourse-webhook-resource/internal/server"
	"github.com/p3ym4n/concourse-webhook-resource/internal/storage"
)

func main() {
	token := os.Getenv("WEBHOOK_TOKEN")
	storagePath := os.Getenv("STORAGE_PATH")
	if storagePath == "" {
		storagePath = "/data/webhooks"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	store, err := storage.NewFileStorage(storagePath)
	if err != nil {
		log.Fatalf("storage init failed: %v", err)
	}

	srv := server.New(store, token)
	log.Printf("webhook server listening on :%s (storage: %s, auth: %v)", port, storagePath, token != "")
	log.Fatal(http.ListenAndServe(":"+port, srv))
}
