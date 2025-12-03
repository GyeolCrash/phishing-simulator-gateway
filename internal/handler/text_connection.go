package handler

import (
	"PishingSimulator_SecurityProject/internal/llm"
	"PishingSimulator_SecurityProject/internal/models"
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

func manageTextSession(conn *websocket.Conn, user models.User, parentCtx context.Context, scenarioKey string) {
	defer conn.Close()
	log.Printf("manageTextSession(): Text session started for user: %s, %s", user.Username, scenarioKey)

	llmSessionID := uuid.New().String()

	// 세션 종료 및 정리
	defer func() {
		log.Printf("manageTextSession(): Clearing session: %s", llmSessionID)
		llm.ClearSession(llmSessionID)
	}()

	// LLM 세션 초기화
	initialUtterance, err := llm.InitSession(llmSessionID, scenarioKey, user.Profile, parentCtx)
	if err != nil {
		log.Printf("manageTextSession(): LLM InitSession failed for user %s: %v", user.Username, err)
		conn.WriteMessage(websocket.TextMessage, []byte("Error initializing session."))
		return
	}

	// 초기 발화 전송
	log.Printf("manageTextSession(): LLM initial utterance for user %s: %s", user.Username, initialUtterance)
	if err := conn.WriteMessage(websocket.TextMessage, []byte(initialUtterance)); err != nil {
		log.Printf("manageTextSession(): Error sending initial utterance to user %s: %v", user.Username, err)
		return
	}

	// Half Duplex 대화 루프
ReadLoop:
	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Error reading message from user %s: %v", user.Username, err)
			break ReadLoop
		}

		if messageType != websocket.TextMessage {
			log.Printf("Unsupported message type from user %s: %d", user.Username, messageType)
			continue
		} else {
			userText := string(message)
			log.Printf("Received text message from user %s: %s", user.Username, userText)

			// LLM에 API를 호출하고 응답을 받는다.
			chatResp, err := llm.Chat(llmSessionID, userText, parentCtx)
			if err != nil {
				log.Printf("LLM Chat failed for user %s: %v", user.Username, err)
				if err := conn.WriteMessage(websocket.TextMessage, []byte("Error processing your message.")); err != nil {
					log.Printf("Error sending error message to user %s: %v", user.Username, err)
					break ReadLoop
				}
				continue
			}

			// LLM 응답을 클라이언트에 전송한다.
			log.Printf("LLM response for user %s: %s", user.Username, chatResp.Response.Utterance)
			if err := conn.WriteMessage(websocket.TextMessage, []byte(chatResp.Response.Utterance)); err != nil {
				log.Printf("Error sending message to user %s: %v", user.Username, err)
				break ReadLoop
			}
			if chatResp.SimulationEnded {
				log.Printf("manageTextSession(): Simulation ended by LLM (Reason: %s)", chatResp.EndReason)
				log.Printf("[시뮬레이션 종료] 점수: %.0f점, 사유: %s, 피드백: %s",
					chatResp.Score,
					chatResp.EndReason,
					chatResp.Feedback,
				)
				conn.WriteMessage(websocket.TextMessage, []byte("[시스템: 통화가 종료되었습니다.]"))
				closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Simulation Ended")
				conn.WriteMessage(websocket.CloseMessage, closeMsg)
				time.Sleep(100 * time.Millisecond)
				break ReadLoop
			}
		}
	}
	log.Printf("Text session ended for user: %s", user.Username)
}
