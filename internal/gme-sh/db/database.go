package db

import (
	"github.com/full-stack-gods/gme.sh-api/pkg/gme-sh/short"
	"log"
)

// PersistentDatabase functions
type PersistentDatabase interface {
	// PersistentDatabase Functions
	SaveShortenedURL(url *short.ShortURL) (err error)
	DeleteShortenedURL(id *short.ShortID) (err error)

	// Database Functions
	FindShortenedURL(id *short.ShortID) (res *short.ShortURL, err error)
	ShortURLAvailable(id *short.ShortID) (available bool)
}

// StatsDatabase functions
type StatsDatabase interface {
	// StatsDatabase Functions
	FindStats(id *short.ShortID) (stats *short.Stats, err error)
	AddStats(id *short.ShortID) (err error)
	DeleteStats(id *short.ShortID) (err error)
}

type PubSub interface {
	Heartbeat() (err error)
	Publish(channel, msg string) (err error)
	Subscribe(c func(channel, payload string), channels ...string) (err error)
	Close() (err error)
}

// Must -> Don't use database, if some error occurred
func MustPersistent(db PersistentDatabase, err error) PersistentDatabase {
	if err != nil {
		log.Fatalln("🚨 Error creating persistent-database:", err)
		return nil
	}
	return db
}

func MustStats(db StatsDatabase, err error) StatsDatabase {
	if err != nil {
		log.Fatalln("🚨 Error creating stats-database:", err)
		return nil
	}
	return db
}

func MustPubSub(db PubSub, err error) PubSub {
	if err != nil {
		log.Fatalln("🚨 Error creating pubsub-database:", err)
		return nil
	}
	return db
}

func shortURLAvailable(db PersistentDatabase, id *short.ShortID) bool {
	if url, _ := db.FindShortenedURL(id); url != nil && !url.IsExpired() {
		return false
	}
	return true
}
