package router

import (
	"backend/internal/account"
	"backend/internal/cache"
	"backend/internal/feed"
	"backend/internal/message"
	"backend/internal/middleware"
	"backend/internal/notification"
	"backend/internal/ratelimit"
	"backend/internal/realtime"
	"backend/internal/social"
	"backend/internal/storage"
	"backend/internal/video"
	"context"
	"time"

	"backend/internal/mq"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func New(
	db *gorm.DB,
	redisClient *cache.Client,
	rabbit *mq.RabbitMQ,
	fileStorage storage.Storage,
) *gin.Engine {
	r := gin.Default()

	loginLimiter := ratelimit.Limit(redisClient, "account_login", 10, time.Minute, ratelimit.KeyByIp)
	registerLimiter := ratelimit.Limit(redisClient, "account_register", 5, time.Hour, ratelimit.KeyByIp)
	accountNameLimiter := ratelimit.Limit(redisClient, "account_name_check", 30, time.Minute, ratelimit.KeyByIp)

	likeLimiter := ratelimit.Limit(redisClient, "like_write", 30, time.Minute, ratelimit.KeyByAccount)
	commentLimiter := ratelimit.Limit(redisClient, "comment_write", 10, time.Minute, ratelimit.KeyByAccount)
	socialLimiter := ratelimit.Limit(redisClient, "social_write", 20, time.Minute, ratelimit.KeyByAccount)

	r.Static("/static", "./.run/uploads")
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	accountRepo := account.NewRepository(db)
	accountService := account.NewService(accountRepo, redisClient, fileStorage)
	accountHandler := account.NewHandler(accountService)

	realtimeHub := realtime.NewHub()
	realtimeTickets := realtime.NewTicketService(redisClient)
	realtimeDispatcher := realtime.NewDispatcher(redisClient, realtimeHub)
	go realtime.StartSubscriber(context.Background(), redisClient, realtimeHub)

	accountGroup := r.Group("/account")
	{
		accountGroup.POST("/register", registerLimiter, accountHandler.Register)
		accountGroup.POST("/login", loginLimiter, accountHandler.Login)
		accountGroup.POST("/checkAccountName", accountNameLimiter, accountHandler.CheckAccountName)
		accountGroup.POST("/findByID", accountHandler.FindByID)
		accountGroup.POST("/findByUsername", accountHandler.FindByUsername)
		accountGroup.POST("/search", accountHandler.Search)
		accountGroup.GET("/avatar/:id", accountHandler.Avatar)
	}
	protectedAccountGroup := accountGroup.Group("")
	protectedAccountGroup.Use(middleware.JWTAuth(accountRepo, redisClient))
	{
		protectedAccountGroup.POST("/me", accountHandler.Me)
		protectedAccountGroup.POST("/logout", accountHandler.Logout)
		protectedAccountGroup.POST("/rename", accountHandler.Rename)
		protectedAccountGroup.POST("/changePassword", accountHandler.ChangePassword)
		protectedAccountGroup.POST("/avatar", accountHandler.UploadAvatar)
	}
	videoRepo := video.NewRepository(db)
	videoService := video.NewService(videoRepo, redisClient, rabbit, fileStorage)
	videoHandler := video.NewHandler(videoService, fileStorage)

	videoGroup := r.Group("/video")
	{
		videoGroup.POST("/getDetail", videoHandler.Detail)
		videoGroup.POST("/listByAuthorID", videoHandler.ListByAuthor)
	}
	protectedVideoGroup := videoGroup.Group("")
	protectedVideoGroup.Use(middleware.JWTAuth(accountRepo, redisClient))
	{
		protectedVideoGroup.POST("/uploadCover", videoHandler.UploadCover)
		protectedVideoGroup.POST("/uploadVideo", videoHandler.UploadVideo)
		protectedVideoGroup.POST("/publish", videoHandler.Publish)
		protectedVideoGroup.POST("/delete", videoHandler.Delete)

		//分片传输路由
		protectedVideoGroup.POST("/uploadChunk", videoHandler.UploadChunk)
		protectedVideoGroup.POST("/chunkStatus", videoHandler.ChunkStatus)
		protectedVideoGroup.POST("/mergeChunks", videoHandler.MergeChunks)
	}
	likeRepo := video.NewLikeRepository(db)

	feedRepo := feed.NewRepository(db)
	feedService := feed.NewService(feedRepo, redisClient, likeRepo, videoRepo, fileStorage)
	feedHandler := feed.NewHandler(feedService)
	feedGroup := r.Group("/feed")
	feedGroup.Use(middleware.SoftJWTAuth(accountRepo, redisClient))
	{
		feedGroup.POST("/listLatest", feedHandler.ListLatest)
		feedGroup.POST("/listByPopularity", feedHandler.ListByPopularity)
		feedGroup.POST("/listLikesCount", feedHandler.ListLikesCount)
	}
	protectedFeedGroup := feedGroup.Group("")
	protectedFeedGroup.Use(middleware.JWTAuth(accountRepo, redisClient))
	{
		protectedFeedGroup.POST("/listByFollowing", feedHandler.ListFollowing)
	}

	likeService := video.NewLikeService(likeRepo, videoRepo, rabbit, fileStorage)
	likeHandler := video.NewLikeHandler(likeService)
	likeGroup := r.Group("/like")
	protectedLikeGroup := likeGroup.Group("")
	protectedLikeGroup.Use(middleware.JWTAuth(accountRepo, redisClient))
	{
		protectedLikeGroup.POST("/like", likeLimiter, likeHandler.Like)
		protectedLikeGroup.POST("/unlike", likeLimiter, likeHandler.UnLike)
		protectedLikeGroup.POST("/isLiked", likeHandler.IsLiked)
		protectedLikeGroup.POST("/listMyLikedVideos", likeHandler.ListMyLikedVideos)
	}

	commentRepo := video.NewCommentRepository(db)
	commentService := video.NewCommentService(commentRepo, videoRepo, rabbit)
	commentHandler := video.NewCommentHandler(commentService)
	commentGroup := r.Group("/comment")
	{
		commentGroup.POST("/listAll", commentHandler.ListAll)
	}
	protectedCommentGroup := commentGroup.Group("")
	protectedCommentGroup.Use(middleware.JWTAuth(accountRepo, redisClient))
	{
		protectedCommentGroup.POST("/publish", commentLimiter, commentHandler.Publish)
		protectedCommentGroup.POST("/delete", commentLimiter, commentHandler.Delete)
	}

	socialRepo := social.NewRepository(db)
	socialService := social.NewService(socialRepo, accountRepo)
	socialHandler := social.NewHandler(socialService)
	socialGroup := r.Group("/social")
	protectedSocialGroup := socialGroup.Group("")
	protectedSocialGroup.Use(middleware.JWTAuth(accountRepo, redisClient))
	{
		protectedSocialGroup.POST("/follow", socialLimiter, socialHandler.Follow)
		protectedSocialGroup.POST("/unfollow", socialLimiter, socialHandler.Unfollow)
		protectedSocialGroup.POST("/getAllFollowers", socialHandler.GetFollowers)
		protectedSocialGroup.POST("/getAllVloggers", socialHandler.GetFollowing)
	}

	notificationRepo := notification.NewRepository(db)
	notificationService := notification.NewService(notificationRepo)
	notificationHandler := notification.NewHandler(notificationService)
	notificationGroup := r.Group("/notification")
	notificationGroup.Use(middleware.JWTAuth(accountRepo, redisClient))
	{
		notificationGroup.POST("/list", notificationHandler.List)
		notificationGroup.POST("/unreadCount", notificationHandler.UnreadCount)
		notificationGroup.POST("/markRead", notificationHandler.MarkRead)
		notificationGroup.POST("/markAllRead", notificationHandler.MarkAllRead)
	}

	messageRepo := message.NewRepository(db)
	messageService := message.NewService(messageRepo, accountRepo, redisClient)
	messageHandler := message.NewHandler(
		messageService,
		realtimeTickets,
		realtimeHub,
		realtimeDispatcher,
	)
	messageGroup := r.Group("/message")
	messageGroup.Use(middleware.JWTAuth(accountRepo, redisClient))
	{
		messageGroup.POST("/listConversations", messageHandler.ListConversations)
		messageGroup.POST("/listMessages", messageHandler.ListMessages)
		messageGroup.POST("/send", messageHandler.Send)
		messageGroup.POST("/markRead", messageHandler.MarkRead)
		messageGroup.POST("/accept", messageHandler.Accept)
		messageGroup.POST("/reject", messageHandler.Reject)
		messageGroup.POST("/block", messageHandler.Block)
		messageGroup.POST("/unblock", messageHandler.Unblock)
		messageGroup.POST("/unreadCount", messageHandler.UnreadCount)
	}
	realtimeGroup := r.Group("/realtime")
	realtimeGroup.Use(middleware.JWTAuth(accountRepo, redisClient))
	{
		realtimeGroup.POST("/wsTicket", messageHandler.IssueTicket)
	}
	r.GET("/ws", messageHandler.WebSocket)

	if rabbit != nil {
		mqHandler := mq.NewHandler(rabbit)
		mqGroup := r.Group("/mq")
		{
			mqGroup.POST("/publish", mqHandler.Publish)
			mqGroup.POST("/publishVideoEvent", mqHandler.PublishVideoEvent)
		}
	}

	return r
}
