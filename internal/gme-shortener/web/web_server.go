package web

import (
	"github.com/full-stack-gods/GMEshortener/internal/gme-shortener/db"
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
	"log"
	"net/http"
)

type WebServer struct {
	db.PersistentDatabase
	redis *redis.Client
}

func (ws *WebServer) Start() {
	router := mux.NewRouter()

	router.HandleFunc("/{id}", ws.handleRedirect)
	router.HandleFunc("/api/v1/create", ws.handleApiV1Create)
	router.HandleFunc("/api/v1/stats/{id}", ws.handleApiV1Stats)
	router.HandleFunc("/api/v1/heartbeat", ws.handleApiV1Heartbeat)

	if err := http.ListenAndServe(":1336", router); err != nil {
		log.Fatalln("Error listening and serving:", err)
	}
}

func NewWebServer(db db.PersistentDatabase, red *redis.Client) *WebServer {
	return &WebServer{
		db,
		red,
	}
}
