package coordination

import (
	"context"
	"crypto/tls"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// ValkeyOptions contains connection settings for a Valkey/Redis-compatible
// coordination service.
type ValkeyOptions struct {
	Addr         string
	Username     string
	Password     string
	DB           int
	UseTLS       bool
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// NewValkeyClient creates a Valkey coordinator backed by go-redis.
func NewValkeyClient(opts ValkeyOptions) (*Valkey, error) {
	if opts.Addr == "" {
		return nil, fmt.Errorf("missing valkey coordination address")
	}
	redisOptions := &redis.Options{
		Addr:         opts.Addr,
		Username:     opts.Username,
		Password:     opts.Password,
		DB:           opts.DB,
		DialTimeout:  opts.DialTimeout,
		ReadTimeout:  opts.ReadTimeout,
		WriteTimeout: opts.WriteTimeout,
	}
	if opts.UseTLS {
		redisOptions.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	return NewValkey(redisPinger{client: redis.NewClient(redisOptions)})
}

var incrementWithExpiryScript = redis.NewScript(`
local current = redis.call("INCR", KEYS[1])
if current == 1 then
	redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
return current
`)

var deleteIfValueScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
end
return 0
`)

type redisPinger struct {
	client *redis.Client
}

func (r redisPinger) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

func (r redisPinger) IncrementWithExpiry(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	milliseconds := ttl.Milliseconds()
	if milliseconds < 1 {
		milliseconds = 1
	}
	return incrementWithExpiryScript.Run(ctx, r.client, []string{key}, strconv.FormatInt(milliseconds, 10)).Int64()
}

func (r redisPinger) SetNXWithExpiry(ctx context.Context, key, value string, ttl time.Duration) (bool, error) {
	return r.client.SetNX(ctx, key, value, ttl).Result()
}

func (r redisPinger) DeleteIfValue(ctx context.Context, key, value string) (bool, error) {
	deleted, err := deleteIfValueScript.Run(ctx, r.client, []string{key}, value).Int64()
	return deleted > 0, err
}

func (r redisPinger) Close() error {
	return r.client.Close()
}
