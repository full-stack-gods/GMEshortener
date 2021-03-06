package db

import (
	"context"
	"encoding/json"
	"github.com/gme-sh/gme.sh-api/pkg/gme-sh/tpl"
	"log"
	"time"

	"github.com/gme-sh/gme.sh-api/internal/gme-sh/config"
	"github.com/gme-sh/gme.sh-api/pkg/gme-sh/short"
	"github.com/go-redis/redis/v8"
)

// PersistentDatabase
// StatsDatabase
type redisDB struct {
	client  *redis.Client
	context context.Context
	ps      *redis.PubSub
}

func newRedisDB(cfg *config.RedisConfig) (*redisDB, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	ctx := context.TODO()
	if res := client.Set(ctx, "heartbeat", 1, 0); res.Err() != nil {
		log.Fatalln("Error connecting to Redis:", res.Err())
		return nil, res.Err()
	}

	return &redisDB{
		client:  client,
		context: ctx,
	}, nil
}

// NewRedisDatabase -> Use Redis as backend
func NewRedisDatabase(cfg *config.RedisConfig) (PersistentDatabase, error) {
	return newRedisDB(cfg)
}

func NewRedisPubSub(cfg *config.RedisConfig) (PubSub, error) {
	return newRedisDB(cfg)
}

func NewRedisStats(cfg *config.RedisConfig) (StatsDatabase, error) {
	return newRedisDB(cfg)
}

/*
 * ==================================================================================================
 *                          D E F A U L T   I M P L E M E N T A T I O N S
 * ==================================================================================================
 */

func (*redisDB) ServiceName() string {
	return "Redis"
}

func (rdb *redisDB) HealthCheck(ctx context.Context) (err error) {
	err = rdb.client.Ping(ctx).Err()
	return
}

/*
 * ==================================================================================================
 *                            P E R M A N E N T  D A T A B A S E
 * ==================================================================================================
 */

func (rdb *redisDB) SaveShortenedURL(short *short.ShortURL) (err error) {
	var data []byte
	data, err = json.Marshal(short)
	if err != nil {
		return
	}
	var exp time.Duration
	if short.ExpirationDate != nil {
		exp = short.ExpirationDate.Sub(time.Now())
	} else {
		exp = redis.KeepTTL
	}
	err = rdb.client.Set(rdb.context, short.ID.RedisKey(), string(data), exp).Err()
	return
}

func (rdb *redisDB) DeleteShortenedURL(id *short.ShortID) (err error) {
	err = rdb.client.Del(rdb.context, id.RedisKey()).Err()
	return
}

func (rdb *redisDB) FindShortenedURL(id *short.ShortID) (res *short.ShortURL, err error) {
	data := rdb.client.Get(rdb.context, id.RedisKey())
	err = data.Err()
	if err != nil {
		return
	}
	err = json.Unmarshal([]byte(data.Val()), &res)
	return
}

func (rdb *redisDB) ShortURLAvailable(id *short.ShortID) bool {
	return shortURLAvailable(rdb, id)
}

/*
 * ==================================================================================================
 *                          E X P I R A T I O N   I M P L E M E N T A T I O N S
 * ==================================================================================================
 */

func (*redisDB) FindExpiredURLs() (res []*short.ShortURL, err error) {
	return []*short.ShortURL{}, nil
}

func (*redisDB) GetLastExpirationCheck() *LastExpirationCheckMeta {
	return &LastExpirationCheckMeta{
		LastCheck: time.Now(),
	}
}

func (*redisDB) UpdateLastExpirationCheck(t time.Time) {
	return
}

/*
 * ==================================================================================================
 *                          T E M P L A T E   I M P L E M E N T A T I O N S
 * ==================================================================================================
 */

func (rdb *redisDB) FindTemplates() (templates []*tpl.Template, err error) {
	cmd := rdb.client.Keys(rdb.context, "tpl::*")
	var keys []string
	if keys, err = cmd.Result(); err != nil {
		return
	}
	for _, k := range keys {
		var val string
		if val, err = rdb.client.Get(rdb.context, k).Result(); err != nil {
			return
		}
		t := new(tpl.Template)
		if err = json.Unmarshal([]byte(val), t); err != nil {
			return
		}
		templates = append(templates, t)
	}
	return
}

func (rdb *redisDB) SaveTemplate(t *tpl.Template) (err error) {
	var data []byte
	data, err = json.Marshal(t)
	if err != nil {
		return
	}
	err = rdb.client.Set(rdb.context, "tpl::"+t.TemplateURL, string(data), 0).Err()
	return
}

/*
 * ==================================================================================================
 *                            S T A T S   D A T A B A S E
 * ==================================================================================================
 */

func (rdb *redisDB) FindStats(id *short.ShortID) (stats *short.Stats, err error) {
	var calls, calls60 uint64
	calls, err = rdb.client.Get(rdb.context, id.RedisKeyf(short.RedisKeyCountGlobal)).Uint64()
	if err != nil {
		return
	}
	calls60, err = rdb.client.Get(rdb.context, id.RedisKeyf(short.RedisKeyCount60)).Uint64()
	if err != nil {
		return
	}
	stats = &short.Stats{
		Calls:   calls,
		Calls60: calls60,
	}
	return
}

func (rdb *redisDB) AddStats(id *short.ShortID) (err error) {
	err = rdb.client.Incr(rdb.context, id.RedisKeyf(short.RedisKeyCountGlobal)).Err()
	if err != nil {
		return
	}
	count60Key := id.RedisKeyf(short.RedisKeyCount60)
	resultExists := rdb.client.Exists(rdb.context, count60Key)
	err = resultExists.Err()
	expire := resultExists.Val() == 0
	if err != nil {
		return
	}
	err = rdb.client.Incr(rdb.context, count60Key).Err()
	if err != nil {
		return
	}
	if !expire {
		err = rdb.client.Expire(rdb.context, count60Key, time.Hour).Err()
	}
	return
}

func (rdb *redisDB) DeleteStats(id *short.ShortID) (err error) {
	err = rdb.client.Del(
		rdb.context,
		id.RedisKeyf(short.RedisKeyCountGlobal),
		id.RedisKeyf(short.RedisKeyCount60),
	).Err()
	return
}

/*
 * ==================================================================================================
 *                                       P U B S U B
 * ==================================================================================================
 */

func (rdb *redisDB) Publish(channel, msg string) (err error) {
	err = rdb.client.Publish(rdb.context, channel, msg).Err()
	return
}

func (rdb *redisDB) Subscribe(c func(channel, payload string), channels ...string) (err error) {
	log.Println("[REDIS] (Re-) Subscribing")
	rdb.ps = rdb.client.Subscribe(rdb.context, channels...)
	// wait for confirmation
	_, err = rdb.ps.Receive(rdb.context)
	if err != nil {
		return
	}
	for msg := range rdb.ps.Channel() {
		c(msg.Channel, msg.Payload)
	}
	// if this range ends, re-subscribe
	return rdb.Subscribe(c, channels...)
}

func (rdb *redisDB) Close() (err error) {
	if rdb.ps == nil {
		return
	}
	err = rdb.ps.Close()
	return
}

/*
 * ==================================================================================================
 *                             P O O L   I M P L E M E N T A T I O N S
 * ==================================================================================================
 */

func (rdb *redisDB) FindPool(id *short.PoolID) (pool *short.Pool, err error) {
	var cmd *redis.StringCmd
	if cmd = rdb.client.Get(rdb.context, "pool::"+id.String()); cmd.Err() != nil {
		return nil, cmd.Err()
	}
	pool = new(short.Pool)
	err = json.Unmarshal([]byte(cmd.String()), pool)
	return
}

func (rdb *redisDB) SavePool(pool *short.Pool) (err error) {
	var data []byte
	if data, err = json.Marshal(pool); err != nil {
		return
	}
	err = rdb.client.Set(rdb.context, "pool::"+pool.ID.String(), string(data), redis.KeepTTL).Err()
	return
}
