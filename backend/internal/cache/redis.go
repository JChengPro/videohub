package cache

import (
	"backend/internal/config"
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type Client struct {
	rdb *redis.Client
}

/*  - New()：连接 Redis
- Get()：读缓存
- Set()：写缓存
*/

func New(cfg config.RedisConfig) (*Client, error) {
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}
	return &Client{rdb: rdb}, nil
}

func (c *Client) Get(ctx context.Context, key string) (string, error) {
	return c.rdb.Get(ctx, key).Result()
}

func (c *Client) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return c.rdb.Set(ctx, key, value, ttl).Err()
}

func (c *Client) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return c.rdb.Del(ctx, keys...).Err()
}

func (c *Client) ScanKeys(ctx context.Context, pattern string) ([]string, error) {
	var cursor uint64
	var allKeys []string
	for {
		keys, nextCursor, err := c.rdb.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, err
		}
		allKeys = append(allKeys, keys...)
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return allKeys, nil
}

func (c *Client) IncrementWithExpire(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	pipe := c.rdb.TxPipeline()

	// 这个 key 的计数 +1
	incrCmd := pipe.Incr(ctx, key)
	//比如窗口设成 1 分钟，那一分钟后这个计数就自动清掉了。
	pipe.Expire(ctx, key, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return incrCmd.Val(), nil
}

// 给 ZSET 里的某个视频热度加分或减分。
func (c *Client) ZIncrBy(ctx context.Context, key string, member uint, delta int64) error {
	return c.rdb.ZIncrBy(ctx, key, float64(delta), strconv.FormatUint(uint64(member), 10)).Err()
}

// 按分数从高到低取排行榜。
func (c *Client) ZRevRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return c.rdb.ZRevRange(ctx, key, start, stop).Result()
}

// 把某个视频从热榜里删掉。
func (c *Client) ZRem(ctx context.Context, key string, member uint) error {
	return c.rdb.ZRem(ctx, key, strconv.FormatUint(uint64(member), 10)).Err()
}
