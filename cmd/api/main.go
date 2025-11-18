package main

import (
	"PishingSimulator_SecurityProject/internal/handler"
	"PishingSimulator_SecurityProject/internal/middleware"
	"PishingSimulator_SecurityProject/internal/storage"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	limit "github.com/yangxikun/gin-limit-by-key"
	"golang.org/x/time/rate"

	_ "PishingSimulator_SecurityProject/docs"

	"github.com/joho/godotenv"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func init() {
	err := godotenv.Load("../../.env")
	if err != nil {
		cwd, _ := os.Getwd()
		log.Println("init(): Error Loading .env, CWD:", cwd, "Error:", err)
	} else {
		log.Println("init(): .env file loaded successfully")
	}
}

// @title Phising Simulator API
// @version 						0.1
// @description
// @host 							localhost:8080
// @BasePath 						/
// @securityDefinitions.apikey		BearerAuth
// @in header
// @name Authorization
// @description Bearer 토큰 형식, Bearer {token}
// @securityDefinitions.apikey AccessCodeAuth
// @in header
// @name Project-Secure
func main() {
	storage.InitDB()
	router := gin.Default()

	// CORS 설정
	config := cors.DefaultConfig()

	// config.AllowAllOrigins = true // 경고: 실제 배포 시에는 특정 도메인으로 제한해야 함
	config.AllowHeaders = append(config.AllowHeaders, "Authorization")
	config.AllowCredentials = true

	router.Use(cors.New(config))

	rateLimitMiddleware := limit.NewRateLimiter(func(c *gin.Context) string {
		return c.ClientIP()
	}, func(c *gin.Context) (*rate.Limiter, time.Duration) {
		return rate.NewLimiter(rate.Every(time.Minute/100), 100), time.Hour
	}, func(c *gin.Context) {
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "Too many requests"})
	})

	router.Use(rateLimitMiddleware)

	// 라우트 설정
	router.POST("/signup", rateLimitMiddleware, middleware.AccessCodeMiddleware(), handler.Signup)
	router.POST("/login", rateLimitMiddleware, middleware.AccessCodeMiddleware(), handler.Login)
	router.GET("/test", rateLimitMiddleware, handler.TestGet)

	// 보호된 라우트 그룹
	protected := router.Group("/api").Use(middleware.AuthMiddleware())
	{
		protected.GET("/profile", handler.Profile)
		protected.GET("/history", handler.GetCallHistory)
		protected.GET("/history/audio/:filename", handler.StreamAudio)
	}

	// WebSocket 핸들러
	router.GET("/ws/simulation", handler.HandleSimulationConnection)

	// Swagger
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	log.Fatal(router.Run(":8080"))
}
