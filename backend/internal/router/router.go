package router

import (
	"backend/internal/account"
	"backend/internal/cache"
	"backend/internal/feed"
	"backend/internal/middleware"
	"backend/internal/ratelimit"
	"backend/internal/social"
	"backend/internal/video"
	"time"

	"backend/internal/mq"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func New(db *gorm.DB, redisClient *cache.Client, rabbit *mq.RabbitMQ) *gin.Engine {
	r := gin.Default()

	loginLimiter := ratelimit.Limit(redisClient, "account_login", 10, time.Minute, ratelimit.KeyByIp)
	registerLimiter := ratelimit.Limit(redisClient, "account_register", 5, time.Hour, ratelimit.KeyByIp)

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
	accountService := account.NewService(accountRepo, redisClient)
	accountHandler := account.NewHandler(accountService)

	accountGroup := r.Group("/account")
	{
		accountGroup.POST("/register", registerLimiter, accountHandler.Register)
		accountGroup.POST("/login", loginLimiter, accountHandler.Login)
		accountGroup.POST("/changePassword", accountHandler.ChangePassword)
		accountGroup.POST("/findByID", accountHandler.FindByID)
		accountGroup.POST("/findByUsername", accountHandler.FindByUsername)
	}
	protectedAccountGroup := accountGroup.Group("")
	protectedAccountGroup.Use(middleware.JWTAuth(accountRepo, redisClient))
	{
		protectedAccountGroup.POST("/me", accountHandler.Me)
		protectedAccountGroup.POST("/logout", accountHandler.Logout)
		protectedAccountGroup.POST("/rename", accountHandler.Rename)
	}
	videoRepo := video.NewRepository(db)
	videoService := video.NewService(videoRepo, redisClient, rabbit)
	videoHandler := video.NewHandler(videoService)

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
	}
	likeRepo := video.NewLikeRepository(db)

	feedRepo := feed.NewRepository(db)
	feedService := feed.NewService(feedRepo, redisClient, likeRepo)
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

	likeService := video.NewLikeService(likeRepo, videoRepo, rabbit)
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
	commentService := video.NewCommentService(commentRepo, rabbit)
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
	socialService := social.NewService(socialRepo)
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
