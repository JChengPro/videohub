package cache

import (
	"backend/internal/config"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/redis/go-redis/v9"
	"strconv"
	"time"
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

// 判断 Redis 返回的错误是不是“key 不存在”。
func IsMiss(err error) bool {
	return err == redis.Nil
}

// 从 Redis 取 []byte，原版缓存 JSON 时喜欢用 []byte。
func (c *Client) GetBytes(ctx context.Context, key string) ([]byte, error) {
	return c.rdb.Get(ctx, key).Bytes()
}

// 把 []byte 写进 Redis。
func (c *Client) SetBytes(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return c.rdb.Set(ctx, key, value, ttl).Err()
}

// 一次性批量获取多个 key，原版 GetVideoByIDs 会用它批量查视频实体缓存。
func (c *Client) MGet(ctx context.Context, keys ...string) ([]interface{}, error) {
	return c.rdb.MGet(ctx, keys...).Result()
}

// token：锁的持有人标识，释放锁时要校验是不是自己加的锁
func randToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (c *Client) Lock(ctx context.Context, key string, ttl time.Duration) (token string, ok bool, err error) {
	token, err = randToken(16)
	if err != nil {
		return "", false, err
	}
	ok, err = c.rdb.SetNX(ctx, key, token, ttl).Result()
	return token, ok, err
}

var unlockScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
else
  return 0
end
`)

func (c *Client) Unlock(ctx context.Context, key string, token string) error {
	_, err := unlockScript.Run(ctx, c.rdb, []string{key}, token).Result()
	return err
}

// 把视频 ID 加入 feed:global_timeline。
func (c *Client) ZAdd(ctx context.Context, key string, members ...redis.Z) error {
	return c.rdb.ZAdd(ctx, key, members...).Err()
}

// 取 ZSET 中最老的一条，判断 Redis 热数据边界。
func (c *Client) ZRangeWithScores(ctx context.Context, key string, start int64, stop int64) ([]redis.Z, error) {
	return c.rdb.ZRangeWithScores(ctx, key, start, stop).Result()
}

func (c *Client) ZRemRangeByRank(ctx context.Context, key string, start int64, stop int64) error {
	return c.rdb.ZRemRangeByRank(ctx, key, start, stop).Err()
}

// 按发布时间倒序取某个时间之前的视频 ID，用于 /feed/latest。
func (c *Client) ZRevRangeByScore(ctx context.Context, key string, max, min string, offset, count int64) ([]string, error) {
	return c.rdb.ZRevRangeByScore(ctx, key, &redis.ZRangeBy{
		Max:    max,
		Min:    min,
		Offset: offset,
		Count:  count,
	}).Result()
}

// 合并多个分钟热榜窗口，用于滑动窗口热榜。
func (c *Client) ZUnionStore(ctx context.Context, dst string, keys []string, aggregate string) error {
	return c.rdb.ZUnionStore(ctx, dst, &redis.ZStore{
		Keys:      keys,
		Aggregate: aggregate,
	}).Err()
}

// 给 key 设置过期时间。
func (c *Client) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return c.rdb.Expire(ctx, key, ttl).Err()
}

func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.rdb.Exists(ctx, key).Result()
	return n > 0, err
}
