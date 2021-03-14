package main

import (
	"fmt"
	"github.com/gme-sh/gme.sh-api/internal/gme-sh/config"
	"github.com/gme-sh/gme.sh-api/internal/gme-sh/db"
	"github.com/gme-sh/gme.sh-api/internal/gme-sh/web"
	"github.com/gofiber/adaptor/v2"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

const (
	// Banner is displayed when the API is started
	Banner = `
                                         /$$                               /$$
                                        | $$                              | $$
 ██████╗ ███╗   ███╗███████╗   /$$$$$$$ | $$$$$$$    /$$$$$$   /$$$$$$   /$$$$$$
██╔════╝ ████╗ ████║██╔════╝  /$$_____/ | $$__  $$  /$$__  $$ /$$__  $$ |_  $$_/
██║  ███╗██╔████╔██║█████╗   |  $$$$$$  | $$  \ $$ | $$  \ $$ | $$  \__/   | $$
██║   ██║██║╚██╔╝██║██╔══╝    \____  $$ | $$  | $$ | $$  | $$ | $$         | $$ /$$
╚██████╔╝██║ ╚═╝ ██║███████╗  /$$$$$$$/ | $$  | $$ |  $$$$$$/ | $$         |  $$$$/
 ╚═════╝ ╚═╝     ╚═╝╚══════╝ |_______/  |__/  |__/  \______/  |__/          \____/`

	// Version of the backend
	Version = "1.0.1-alpha" // semantic
)

func main() {
	fmt.Println(Banner)
	fmt.Println("Starting $GMEshort", Version, "🚀")
	fmt.Println()

	//// Config
	log.Println("└ Loading config")
	cfg := config.LoadConfig()
	if cfg == nil {
		return
	}
	////

	//// Database
	// persistentDB is used to store short urls (persistent, obviously)
	var persistentDB db.PersistentDatabase
	// statsDB is used to store temporary information for short urls (eg. stats, caching)
	var statsDB db.StatsDatabase
	// pubSub is used for PubSub // (SharedCache)
	var pubSub db.PubSub
	var cache db.DBCache

	// PubSub Backend
	switch strings.ToLower(cfg.Backends.PubSubBackend) {
	case "":
		log.Println("👉 No pubsub backend selected")
		break
	case "redis":
		log.Println("👉 Using Redis as pubsub-backend")
		// TODO
		pubSub = db.MustPubSub(db.NewRedisPubSub(cfg.Database.Redis))
		break
	default:
		log.Fatalln("🚨 Unknown pubsub backend:", cfg.Backends.PubSubBackend)
		return
	}

	// Stats Backend
	switch strings.ToLower(cfg.Backends.StatsBackend) {
	case "redis":
		log.Println("👉 Using Redis as stats-backend")
		statsDB = db.MustStats(db.NewRedisStats(cfg.Database.Redis))
		break
	default:
		log.Fatalln("🚨 Unknown stats backend:", cfg.Backends.StatsBackend)
		return
	}

	// Cache Backend
	switch strings.ToLower(cfg.Backends.CacheBackend) {
	case "local":
		log.Println("👉 Using local cache")
		cache = db.NewLocalCache()
		break
	case "shared":
		if pubSub == nil {
			log.Fatalln("🚨 You need to select a valid pubsub backend to use shared cache")
			return
		}
		log.Println("👉 Using shared cache")
		cache = db.NewSharedCache(pubSub)
		break
	default:
		log.Fatalln("🚨 Unknown cache backend:", cfg.Backends.StatsBackend)
		return
	}

	// Persistent Backend
	switch strings.ToLower(cfg.Backends.PersistentBackend) {
	case "bbolt":
		log.Println("👉 Using BBolt as persistent-backend")
		persistentDB = db.MustPersistent(db.NewBBoltDatabase(cfg.Database.BBolt, cache))
		break
	case "mongo":
		log.Println("👉 Using MongoDB as persistent-backend")
		persistentDB = db.MustPersistent(db.NewMongoDatabase(cfg.Database.Mongo, cache))
		break
	case "redis":
		log.Println("👉 Using Redis as persistent-backend")
		persistentDB = db.MustPersistent(db.NewRedisDatabase(cfg.Database.Redis))
		break
	default:
		log.Fatalln("🚨 Unknown persistent backend:", cfg.Backends.PersistentBackend)
		return
	}

	////////////////////////////////////////////////////////////////////////////////////////

	if cache != nil {
		log.Println("👉 Subscribing pubsub ...")
		if _, ok := cache.(*db.SharedCache); ok {
			// subscribe to shared cache
			// e. g. Redis Pub-Sub
			go func() {
				log.Println("SCACHE :: Subscribing to redis channels ...")
				if err := cache.(*db.SharedCache).Subscribe(); err != nil {
					log.Println("SCACHE :: Error:", err)
				}
			}()
		}
	}

	////////////////////////////////////////////////////////////////////////////////////////

	// Expiration check
	ex := db.NewExpirationCheck(cfg.ExpirationCheckInterval.Duration, cfg.ExpirationDryRun, persistentDB)
	exc := make(chan bool, 1)
	go ex.Start(exc)

	////////////////////////////////////////////////////////////////////////////////////////

	health, err := db.NewHealthCheck(persistentDB, statsDB, pubSub)
	if err != nil {
		log.Fatalln("Error creating health check:", err)
		return
	}

	////

	//// Web-Server
	server := web.NewWebServer(persistentDB, statsDB, cfg)
	// stats
	server.App.Get("/health", adaptor.HTTPHandler(health.Handler()))

	/// Templates
	// find templates
	templates, err := persistentDB.FindTemplates()
	if err != nil {
		log.Fatalln("Loading templates failed:", err)
		return
	}
	for _, t := range templates {
		t.Check()
		t.Register(server.App)
	}
	///
	go server.Start()
	////

	log.Println("WebServer is (hopefully) up and running")
	log.Println("Press CTRL+C to exit gracefully")

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// cancel expiration
	exc <- true

	// after CTRL+c
	if pubSub != nil {
		log.Println("Shutting down pubsub")
		if err := pubSub.Close(); err != nil {
			log.Println("  🤬", err)
		}
	}
}
