package web

import (
	"github.com/full-stack-gods/GMEshortener/internal/gme-shortener/db"
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
	"log"
	"net/http"
)

type WebServer struct {
	db.Database
	redis *redis.Client
}

func (ws *WebServer) Start() {
	router := mux.NewRouter()

	router.HandleFunc("/{id}", ws.handleRedirect)
	//router.HandleFunc("/404/{b64id}", ws.handleShortURLNotFound)
	router.HandleFunc("/api/create", ws.handleShortCreate)
	//router.HandleFunc("/", ws.handleIndex)

	if err := http.ListenAndServe(":1336", router); err != nil {
		log.Fatalln("Error listening and serving:", err)
	}
}

func NewWebServer(db db.Database, red *redis.Client) *WebServer {
	return &WebServer{
		db,
		red,
	}
}
