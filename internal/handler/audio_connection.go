package handler

import (
	"PishingSimulator_SecurityProject/internal/archiver"
	"PishingSimulator_SecurityProject/internal/models"
	"PishingSimulator_SecurityProject/internal/storage"
	"fmt"
	"path/filepath"
	"time"

	"context"
	"log"
	"os"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

func manageAudioSession(conn *websocket.Conn, user models.User, parentCtx context.Context, scenarioKey string) {
	defer conn.Close()
	log.Printf("Audio session started for user: %s", user.Username)

	sessionID := uuid.New().String()

	// context
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	sessionStartTime := time.Now()

	// WaitGroup for goroutines
	var wg sync.WaitGroup
	wg.Add(4)

	const DATA_SIZE = 256

	clientChan := make(chan []byte, DATA_SIZE)
	serverChan := make(chan []byte, DATA_SIZE)
	archiveC2SChan := make(chan []byte, DATA_SIZE)
	archiveS2CChan := make(chan archiver.ArchiveS2CJob, DATA_SIZE)

	archiver, err := archiver.NewArchiver(sessionID)
	if err != nil {
		log.Printf("manageAudioSession(): Failed to crate archiver: %v", err)
		return
	}
	defer archiver.CloseBaseTrack()

	// Client -> Server, 읽기 전담
	go func() {
		defer wg.Done()
		defer cancel()
		clientReadPump(conn, user.Username, clientChan, ctx)
	}()

	// Server -> Client, 쓰기 전담
	go func() {
		defer wg.Done()
		defer cancel()
		clientWritePump(conn, user.Username, serverChan, ctx)
	}()

	// STT/LLM/TTS
	go func() {
		defer wg.Done()
		defer cancel()
		orchestrateAudioSession(
			user,
			scenarioKey,
			sessionID,
			sessionStartTime,
			clientChan,
			serverChan,
			archiveC2SChan,
			archiveS2CChan,
			ctx,
		)
		//orchestrateVoiceEchoTest(user, sessionStartTime, clientChan, serverChan, archiveC2SChan,
		//	archiveS2CChan, ctx)
	}()

	go func() {
		defer wg.Done()
		defer cancel()
		archiveAudioConversation(user.Username, archiver, archiveC2SChan, archiveS2CChan, ctx)
	}()

	wg.Wait()

	/* 세션 종료 후 오디오 병합 */
	log.Printf("Audio Session ended for user %s, Archiving audio files...", user.Username)

	finalDir := filepath.Join("data", "Records", user.Username)
	if err := os.MkdirAll(finalDir, 0755); err != nil {
		log.Printf("manageAudioSession(): Failed to create completed Record dir: %v", err)
		return
	}
	finalFilePath := filepath.Join(finalDir, fmt.Sprintf("%s.mp3", sessionID))

	if err := archiver.MergeAndSave(finalFilePath); err != nil {
		log.Printf("manageAudioSession(): Failed to merge audio files: %v", err)
		return
	}

	userID, err := storage.GetUserIDByUsername(user.Username)
	if err != nil {
		log.Printf("manageAudioSession(): Failed to get user ID for archiving: %v", err)
		return
	}

	if err := storage.CreateRecords(userID, scenarioKey, finalFilePath); err != nil {
		log.Printf("manageVoiceSession(): Failed to save Record to database: %v", err)
	} else {
		log.Printf("manageVoiceSession(): Successfully saved Record metadata to DB for user: %s, path: %s", user.Username, finalFilePath)
	}
}

func clientReadPump(conn *websocket.Conn, username string, clientChan chan<- []byte, ctx context.Context) {
	log.Printf("clientReadPump(): started for user: %s", username)
	defer close(clientChan)
	for {
		select {
		case <-ctx.Done():
			log.Printf("clientReadPump(): Canceled with %s", username)
			return
		default:
		}
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("clientReadPump(): Error reading message from user %s: %v", username, err)
			return
		}

		if messageType != websocket.BinaryMessage {
			log.Printf("clientReadPump(): Unsupported message type from user %s: %d", username, messageType)
			continue
		} else {
			// log.Printf("clientReadPump(): Received audio message from user %s: %d bytes", username, len(message))
			clientChan <- message
		}

	}
}

func clientWritePump(conn *websocket.Conn, username string, clientAudioOutChan <-chan []byte, ctx context.Context) {
	log.Printf("clientWritePump(): started for user: %s", username)
	for {
		select {
		case <-ctx.Done():
			log.Printf("clientWritePump(): %s", username)
			conn.WriteMessage(websocket.CloseMessage, []byte{})
			return

		case audioData, ok := <-clientAudioOutChan:
			if !ok {
				log.Printf("clientWritePump(): audio out Chan closed for user: %s", username)
				//conn.WriteMessage(websocket.CloseMessage, []byte{})
				closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Session Ended")
				if err := conn.WriteMessage(websocket.CloseMessage, closeMsg); err != nil {
					log.Printf("clientWritePump(): Failed to send close message: %v", err)
				}
				return
			}

			if err := conn.WriteMessage(websocket.BinaryMessage, audioData); err != nil {
				log.Printf("clientWritePump(): Error sending audio to user %s: %v", username, err)
				return
			}
			log.Printf("clientWritePump(): Sent audio to user %s: %d bytes", username, len(audioData))
		}
	}
}
