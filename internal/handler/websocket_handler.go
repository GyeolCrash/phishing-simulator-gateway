package handler

import (
	"PishingSimulator_SecurityProject/internal/auth"
	"PishingSimulator_SecurityProject/internal/models"
	"PishingSimulator_SecurityProject/internal/storage"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// Upgrade HTTP connection to WebSocket
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,

	CheckOrigin: func(r *http.Request) bool {
		expectedSecret := os.Getenv("PROJECT_ACCESS_CODE")
		if expectedSecret == "" {
			log.Println("Code is not set")
			return false
		}
		receivedSecret := r.Header.Get("Project-Secret")
		return receivedSecret == expectedSecret
	},
}

type InitMessage struct {
	Type     string `json:"type"`
	Token    string `json:"token"`
	Scenario string `json:"scenario"`
	Mode     string `json:"mode"`
}

// HandleSimulationConnection godoc
// @Summary      보이스피싱 시뮬레이션 시작 (WebSocket)
// @Description  지정된 시나리오와 모드로 실시간 시뮬레이션을 위한 WebSocket 연결을 시작합니다.
// @Description  <br>
// @Description  **[중요]** 이것은 표준 HTTP API가 아닙니다. `ws://` 또는 `wss://` 스킴을 사용해야 합니다.
// @Description  <br>
// @Description  **[인증 및 초기화]**
// @Description  WebSocket 연결이 성공하면, 클라이언트는 **반드시 첫 번째 메시지**로 다음 구조의 JSON을 전송해야 합니다.
// @Description  <pre><code>{
// @Description    "type": "init",
// @Description    "token": "YOUR_JWT_TOKEN_HERE",
// @Description    "scenario": "loan_scam",
// @Description    "mode": "voice"
// @Description  }</code></pre>
// @Tags         Simulation (WebSocket)
// @Accept       json
// @Produce      json
// @in header        Project-Secure header    string  true  "프로젝트 접근 보안 코드"
// @Success      101      {string}  string  "Switching Protocols"
// @Failure      400      {object}  map[string]string "잘못된 초기 메시지 (예: type != 'init', 잘못된 시나리오/모드)"
// @Failure      401      {object}  map[string]string "인증 실패 (초기 메시지의 토큰이 유효하지 않음)"
// @Router       /ws/simulation [get]
func HandleSimulationConnection(c *gin.Context) {
	// WebSocket 연결 업그레이드과 종료
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("HandleSimulationConnection(): Failed to upgrade to WebSocket : %v", err)
		return
	}
	defer conn.Close()
	conn.SetReadLimit(10485760) // DoS 방지용 최대 메시지 크기 제한, 10MB

	var initMsg InitMessage
	if err := conn.ReadJSON(&initMsg); err != nil {
		log.Println("HandleSimulationConnection(): Failed to read init messsage", err)
		conn.WriteJSON(gin.H{"error": "Invalid init message"})
		return
	}

	if initMsg.Type != "init" {
		conn.WriteJSON(gin.H{"error": "Init messsage required"})
		return
	}

	// 사용자 토큰 검증
	claims, err := auth.ValidateToken(initMsg.Token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}

	username := claims.Username
	scenarioKey := initMsg.Scenario
	mode := initMsg.Mode

	log.Printf("User %s connected with scenario key: %s", username, scenarioKey)

	// 시나리오와 모드 검증
	scenario, exists := models.GetScenario(scenarioKey)
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid scenario key"})
		return
	}
	if mode != "text" && mode != "voice" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid mode"})
		return
	}

	user, err := storage.GetUserByUsername(username)
	if err != nil {
		log.Printf("HandleSimulationConnection(): Failed to get user info for websocket: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve user"})
		return
	}

	log.Printf("User: %s, %d, %s, Scenario: %s, Mode: %s", user.Profile.Name, user.Profile.Age, user.Profile.Gender, scenario.Name, mode)

	log.Printf("WebSocket connection established for user: %s", username)

	// 초기 메시지 전송
	initalMessage := fmt.Sprintf("Start Secnario %s: %s", scenario.Name, scenario.Description)
	if err := conn.WriteMessage(websocket.TextMessage, []byte(initalMessage)); err != nil {
		log.Printf("Error sending message to user %s: %v", username, err)
		return
	}

	// 모드에 따른 세션 관리
	switch mode {
	case "text":
		manageTextSession(conn, user, context.Background(), scenarioKey)
	case "voice":
		manageAudioSession(conn, user, context.Background(), scenarioKey)
	default:
		// add error handling for unsupported mode
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		log.Printf("Unsupported mode for user %s: %s", username, mode)
	}
}
