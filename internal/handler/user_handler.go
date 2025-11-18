/**
* Name: 			auth_handler.go
* Description: 		Gin 프레임워크의 HTTP 핸들러
* Workflow: 		회원가입, 로그인, 프로필 조회
 */
package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"PishingSimulator_SecurityProject/internal/auth"
	"PishingSimulator_SecurityProject/internal/models"
	"PishingSimulator_SecurityProject/internal/storage"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// /Signup 요청 바디
type SignupRequest struct {
	Username string             `json:"username" example:"new_user"`
	Password string             `json:"password" example:"password123"`
	Profile  models.UserProfile `json:"profile"`
}

// /Login 요청 바디
type LoginRequest struct {
	Username string `json:"username" example:"my_user"`
	Password string `json:"password" example:"password123"`
}

type SuccessResponse struct {
	Message string `json:"message" example:"User created successfully"`
}
type ErrorResponse struct {
	Error string `json:"error" example:"에러 원인 및 설명"`
}
type LoginSuccessResponse struct {
	Token string `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
}

// 프로필 조회 응답
type ProfileResponse struct {
	Message  string `json:"message" example:"this is a protected profile"`
	Username string `json:"username" example:"gildong"`
}

// 통화 기록 목록 응답 (Wrapper)
type HistoryResponse struct {
	History []models.Record `json:"history"`
}

// Signup godoc
// @Summary      회원가입 (Signup)
// @Description  새로운 사용자 계정을 생성합니다.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Security	 AccessCodeAuth
// @Param        request body handler.SignupRequest true "회원가입 요청 정보"
// @Success      200 {object} handler.SuccessResponse
// @Failure      400 {object} handler.ErrorResponse
// @Failure      500 {object} handler.ErrorResponse
// @Router       /signup [post]
func Signup(c *gin.Context) {
	var credentials SignupRequest

	rawData, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	if err := json.Unmarshal(rawData, &credentials); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if strings.TrimSpace(credentials.Username) == "" || strings.TrimSpace(credentials.Password) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username and Password cannot be empty"})
		return
	}
	if credentials.Profile.Age <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Age must be a positive number"})
		return
	}

	// password 해싱
	HashedPassword, err := bcrypt.GenerateFromPassword([]byte(credentials.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}
	// DB에 사용자 생성
	if err := storage.CreateUser(credentials.Username, string(HashedPassword), credentials.Profile); err != nil {
		if errors.Is(err, storage.ErrUsernameExists) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Username already exists"})
		} else {
			log.Printf("[ERROR] Failed to create user (database error): %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user (database error)"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User created successfully"})

}

func TestGet(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"good": "ok"})
}

// @Summary      로그인
// @Description  사용자 로그인을 처리합니다.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Security	 AccessCodeAuth
// @Param        request body handler.LoginRequest true "로그인 요청 정보"
// @Success      200 {object} handler.LoginSuccessResponse
// @Failure      400 {object} handler.ErrorResponse "잘못된 요청"
// @Failure      401 {object} handler.ErrorResponse "인증 실패 (자격 증명 오류)"
// @Failure      500 {object} handler.ErrorResponse "서버 내부 오류"
// @Router       /login [post]
func Login(c *gin.Context) {
	var credentials LoginRequest

	rawData, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read request body"})
		return
	}
	if err := json.Unmarshal(rawData, &credentials); err != nil {
		log.Printf("[ERROR] Login: json.Unmarshal failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "JSON parsing error: " + err.Error()})
		return
	}

	if credentials.Username == "" || credentials.Password == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	user, err := storage.GetUserByUsername(credentials.Username)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}
		log.Printf("[ERROR] GetUserByUsername failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(credentials.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	tokenString, err := auth.GenerateToken(credentials.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": tokenString})
}

// Profile godoc
// @Summary      프로필 조회 (Profile)
// @Description  인증된 사용자의 프로필 정보를 조회합니다. (JWT 필요)
// @Tags         API (Protected)
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} object{message=string, username=string}
// @Failure      401 {object} handler.ErrorResponse "인증 토큰 누락 또는 만료"
// @Router       /api/profile [get]
func Profile(c *gin.Context) {
	username, _ := c.Get("username")
	c.JSON(http.StatusOK, gin.H{"message": "this is a protected profile", "username": username})
}

// GetCallHistory godoc
// @Summary      사용자 통화 기록 조회
// @Description  사용자의 과거 시뮬레이션(통화/채팅) 기록 목록을 최신순으로 반환합니다.
// @Tags         API (Protected)
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} handler.HistoryResponse "history: [기록 배열]"
// @Failure      401 {object} handler.ErrorResponse "인증 실패"
// @Failure      500 {object} handler.ErrorResponse "DB 조회 실패 등 서버 오류"
// @Router       /api/history [get]
func GetCallHistory(c *gin.Context) {
	username := c.GetString("username")

	userID, err := storage.GetUserIDByUsername(username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
		return
	}

	records, err := storage.GetRecordsByUserID(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch records"})
		return
	}
	c.JSON(http.StatusOK, HistoryResponse{History: records})
}

// StreamAudio godoc
// @Summary      녹음된 오디오 파일 스트리밍
// @Description  특정 통화 기록의 오디오 파일(.mp3)을 재생합니다.
// @Description  <br> **[인증]** Header에 `Authorization: Bearer ...`를 넣거나, URL 파라미터 `?token=...`을 사용하세요.
// @Tags         API (Protected)
// @Produce      audio/mpeg
// @Security     BearerAuth
// @Param        filename path      string  true  "오디오 파일명 (예: session_uuid.mp3)"
// @Param        token    query     string  false "JWT 토큰 (Header 사용 불가 시)"
// @Success      200      {file}    file    "오디오 바이너리 데이터"
// @Failure      401      {object}  handler.ErrorResponse "인증 실패"
// @Failure      404      {object}  handler.ErrorResponse "해당 파일을 찾을 수 없음"
// @Router       /api/history/audio/{filename} [get]
func StreamAudio(c *gin.Context) {
	username := c.GetString("username")
	filename := c.Param("filename")

	cleanFilename := filepath.Base(filename)
	filePath := filepath.Join("data", "records", username, cleanFilename)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Audio file not found"})
		return
	}

	c.File(filePath)
}
