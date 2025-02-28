package pkg

import (
	"context"

	"github.com/go-redis/redis/v8"
	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v8"
)

func InitRedis() (*redis.Client, *redsync.Redsync, context.Context) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // Redis 服务器地址
		Password: "",               // 密码
		DB:       0,                // 默认数据库
	})

	var ctx = context.Background()
	// 测试连接
	pong, err := rdb.Ping(ctx).Result()
	if err != nil {
		Log.Fatalf("failed to connect redis: %v", err)
		panic("failed to connect redis")
	}

	Log.Printf("Redis connected: %s", pong)

	// 初始化 redsync
	pool := goredis.NewPool(rdb)
	rs := redsync.New(pool)
	return rdb, rs, ctx
}
