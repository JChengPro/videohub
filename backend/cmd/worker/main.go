package main

import (
	"backend/internal/cache"
	"backend/internal/config"
	"backend/internal/db"
	"backend/internal/mq"
	"backend/internal/observability"
	"backend/internal/video"
	"backend/internal/worker"
	"context"
	"encoding/json"
	"log"
	"time"
)

func main() {
	cfg, usedDefault, err := config.LoadLocalDev("configs/config.yaml")
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}
	log.Printf("worker config loaded, usedDefault=%v", usedDefault)

	workerPprof, err := observability.NewPprofServer(
		"worker",
		cfg.Observability.Pprof.Enabled,
		cfg.Observability.Pprof.WorkerAddr,
	)
	if err != nil {
		log.Fatalf("start worker pprof failed: %v", err)
	}
	defer workerPprof.Close()

	rabbit, err := mq.NewRabbitMQ(cfg.RabbitMQ)
	if err != nil {
		log.Fatalf("connect rabbitmq failed: %v", err)
	}
	defer rabbit.Close()

	//  worker 启动时也连接 Redis
	//如果 Redis 连不上，worker 仍然可以消费消息，只是不做缓存删除
	redisClient, err := cache.New(cfg.Redis)
	if err != nil {
		log.Printf("redis unavailable, cache invalidation disabled: %v", err)
		redisClient = nil
	}
	videoWorker := worker.NewVideoWorker(redisClient)

	//API 进程需要连 MySQL
	//worker 进程也需要连 MySQL
	sqlDB, err := db.New(cfg.Database)
	if err != nil {
		log.Fatalf("connect mysql failed: %v", err)
	}
	videoRepo := video.NewRepository(sqlDB)
	// worker 启动后，同时启动 outbox poller。
	//poller 每 2 秒扫描一次 outbox_msgs 表。
	//如果发现 pending 消息，就发到 RabbitMQ。
	worker.StartOutboxPoller(context.Background(), videoRepo, rabbit, 2*time.Second)

	likeWorker := worker.NewLikeWorker(videoRepo, redisClient)

	commentWorker := worker.NewCommentWorker(videoRepo, redisClient)

	go consumeVideoPublished(rabbit, videoWorker)
	go consumeLike(rabbit, likeWorker)
	go consumeComment(rabbit, commentWorker)

	log.Println("worker started, waiting message...")
	select {} //永远阻塞，让主 goroutine 不退出
}

func consumeVideoPublished(rabbit *mq.RabbitMQ, videoWorker *worker.VideoWorker) {
	if err := rabbit.DeclareQueue(mq.VideoPublishedQueueName); err != nil {
		log.Fatalf("declare video queue failed: %v", err)
	}

	deliveries, err := rabbit.Consume(mq.VideoPublishedQueueName)
	if err != nil {
		log.Fatalf("consume video queue failed: %v", err)
	}

	log.Println("video worker started")

	for d := range deliveries {
		var event mq.VideoPublishedEvent
		if err := json.Unmarshal(d.Body, &event); err != nil {
			log.Printf("invalid video message: %s", string(d.Body))
			d.Nack(false, false)
			continue
		}

		switch event.EventType {
		case "video_published", "video_deleted":
			if err := videoWorker.HandleVideoPublished(context.Background(), event); err != nil {
				log.Printf("handle video event failed: %v", err)
				d.Nack(false, true)
				continue
			}
		default:
			log.Printf("unknown video event type: %s", event.EventType)
			d.Nack(false, false)
			continue
		}

		if err := d.Ack(false); err != nil {
			log.Printf("ack video message failed: %v", err)
		}
	}
}

func consumeLike(rabbit *mq.RabbitMQ, likeWorker *worker.LikeWorker) {
	if err := rabbit.DeclareQueue(mq.LikeQueueName); err != nil {
		log.Fatalf("declare like queue failed: %v", err)
	}
	deliveries, err := rabbit.Consume(mq.LikeQueueName)
	if err != nil {
		log.Fatalf("consume like queue failed: %v", err)
	}
	log.Println("like worker started")

	for d := range deliveries {
		var event mq.LikeEvent
		if err := json.Unmarshal(d.Body, &event); err != nil {
			log.Printf("invalid like message: %s", string(d.Body))
			d.Nack(false, false)
			continue
		}
		switch event.EventType {
		case "like_created":
			if err := likeWorker.HandleLikeCreated(context.Background(), event); err != nil {
				log.Printf("handle like created failed: %v", err)
				d.Nack(false, true)
				continue
			}
		case "like_deleted":
			if err := likeWorker.HandleLikeDeleted(context.Background(), event); err != nil {
				log.Printf("handle like deleted failed: %v", err)
				d.Nack(false, true)
				continue
			}
		default:
			log.Printf("unknown like event type: %s", event.EventType)
			d.Nack(false, false)
			continue
		}
		if err := d.Ack(false); err != nil {
			log.Printf("ack like message failed: %v", err)
		}
	}
}

func consumeComment(rabbit *mq.RabbitMQ, commentWorker *worker.CommentWorker) {
	if err := rabbit.DeclareQueue(mq.CommentQueueName); err != nil {
		log.Fatalf("declare comment queue failed: %v", err)
	}
	deliveries, err := rabbit.Consume(mq.CommentQueueName)
	if err != nil {
		log.Fatalf("consume comment queue failed: %v", err)
	}
	log.Println("comment worker started")

	for d := range deliveries {
		var event mq.CommentEvent
		if err := json.Unmarshal(d.Body, &event); err != nil {
			log.Printf("invalid comment message: %s", string(d.Body))
			d.Nack(false, false)
			continue
		}
		switch event.EventType {
		case "comment_published":
			if err := commentWorker.HandleCommentPublished(context.Background(), event); err != nil {
				log.Printf("handle comment published failed: %v", err)
				d.Nack(false, true)
				continue
			}
		case "comment_deleted":
			if err := commentWorker.HandleCommentDeleted(context.Background(), event); err != nil {
				log.Printf("handle comment deleted failed: %v", err)
				d.Nack(false, true)
				continue
			}
		default:
			log.Printf("unknown comment event type: %s", event.EventType)
			d.Nack(false, false)
			continue
		}
		if err := d.Ack(false); err != nil {
			log.Printf("ack comment message failed: %v", err)
		}
	}
}

func consumeSocial(rabbit *mq.RabbitMQ, socialWorker *worker.SocialWorker) {
	if err := rabbit.DeclareQueue(mq.SocialQueueName); err != nil {
		log.Fatalf("declare social queue failed: %v", err)
	}

	deliveries, err := rabbit.Consume(mq.SocialQueueName)
	if err != nil {
		log.Fatalf("consume social queue failed: %v", err)
	}

	log.Println("social worker started")

	for d := range deliveries {
		var event mq.SocialEvent
		if err := json.Unmarshal(d.Body, &event); err != nil {
			log.Printf("invalid social message: %s", string(d.Body))
			d.Nack(false, false)
			continue
		}

		switch event.EventType {
		case "social_followed":
			if err := socialWorker.HandleSocialFollowed(context.Background(), event); err != nil {
				log.Printf("handle social followed failed: %v", err)
				d.Nack(false, true)
				continue
			}
		case "social_unfollowed":
			if err := socialWorker.HandleSocialUnfollowed(context.Background(), event); err != nil {
				log.Printf("handle social unfollowed failed: %v", err)
				d.Nack(false, true)
				continue
			}
		default:
			log.Printf("unknown social event type: %s", event.EventType)
			d.Nack(false, false)
			continue
		}

		if err := d.Ack(false); err != nil {
			log.Printf("ack social message failed: %v", err)
		}
	}
}
