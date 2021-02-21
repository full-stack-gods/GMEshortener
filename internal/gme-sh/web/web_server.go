package web

import (
	"github.com/full-stack-gods/gme.sh-api/internal/gme-sh/config"
	"github.com/full-stack-gods/gme.sh-api/internal/gme-sh/db"
	"github.com/full-stack-gods/gme.sh-api/pkg/gme-sh/short"
	"github.com/gorilla/mux"
	"log"
	"net/http"
)

type WebServer struct {
	db.PersistentDatabase
	db.TemporaryDatabase
	config *config.Config
}

func (ws *WebServer) Start() {
	router := mux.NewRouter()

	router.HandleFunc("/gme-sh-block", ws.handleGMEBlock)
	router.HandleFunc("/api/v1/create", ws.handleApiV1Create).Methods("POST")
	router.HandleFunc("/api/v1/stats/{id}", ws.handleApiV1Stats).Methods("GET")
	router.HandleFunc("/api/v1/heartbeat", ws.handleApiV1Heartbeat).Methods("GET")
	router.HandleFunc("/api/v1/{id}/{secret64}", ws.handleApiV1Delete).Methods("DELETE")
	router.HandleFunc("/{id}", ws.handleRedirect)

	log.Println("🌎 Binding", ws.config.WebServer.Addr, "...")
	if err := http.ListenAndServe(ws.config.WebServer.Addr, router); err != nil {
		log.Fatalln("    └ ❌ FAILED:", err)
	}
}

func NewWebServer(persistent db.PersistentDatabase, temporary db.TemporaryDatabase, cfg *config.Config) *WebServer {
	return &WebServer{
		persistent,
		temporary,
		cfg,
	}
}

func (ws *WebServer) FindShort(id *short.ShortID) (url *short.ShortURL, err error) {
	if ws.TemporaryDatabase != nil {
		url, err = ws.TemporaryDatabase.FindShortenedURL(id)
	}
	if url == nil || err != nil {
		url, err = ws.PersistentDatabase.FindShortenedURL(id)
	}
	return
}

func (ws *WebServer) DeleteShort(id *short.ShortID) (persError error, tempError error) {
	if ws.TemporaryDatabase != nil {
		tempError = ws.TemporaryDatabase.DeleteShortenedURL(id)
	}
	persError = ws.PersistentDatabase.DeleteShortenedURL(id)
	return
}

func (ws *WebServer) ShortAvailable(id *short.ShortID, temp bool) bool {
	if temp && ws.TemporaryDatabase != nil {
		return ws.TemporaryDatabase.ShortURLAvailable(id)
	} else {
		return ws.PersistentDatabase.ShortURLAvailable(id)
	}
}
