package handler

import (
	"PishingSimulator_SecurityProject/internal/archiver"
	"PishingSimulator_SecurityProject/internal/llm"
	"PishingSimulator_SecurityProject/internal/models"
	"strings"
	"time"

	"context"
	"log"

	"github.com/google/uuid"
)

func orchestrateAudioSession(
	user models.User,
	scenarioKey string, // [추가] 시나리오 키
	sessionStartTime time.Time,
	clientChan <-chan []byte,
	serverChan chan<- []byte,
	archiveC2SChan chan<- []byte,
	archiveS2CChan chan<- archiver.ArchiveS2CJob,
	parentCtx context.Context,
) {
	username := user.Username
	llmSessionID := uuid.New().String() // [추가] LLM 세션 ID 생성
	log.Printf("orchestrateAudioSession(): started for user: %s, session: %s", username, llmSessionID)

	// 채널 정리
	defer close(serverChan)
	defer close(archiveC2SChan)
	defer close(archiveS2CChan)

	sttRecognizer, err := llm.NewStreamingRecognizer(parentCtx)
	if err != nil {
		log.Printf("orchestrateAudioSession(): Failed to create STT: %v", err)
		return
	}

	ttsClient, err := llm.NewTTSClient(parentCtx)
	if err != nil {
		log.Printf("orchestrateAudioSession(): Failed to create TTS: %v", err)
		sttRecognizer.Close() // TTS 실패 시 STT도 닫고 종료
		return
	}

	// 2. 리소스 정리 (defer)
	defer func() {
		log.Printf("orchestrateAudioSession(): Cleaning up resources for %s", username)
		if err := sttRecognizer.Close(); err != nil {
			log.Printf("orchestrateAudioSession(): Error closing STT: %v", err)
		}
		if err := ttsClient.Close(); err != nil {
			log.Printf("orchestrateAudioSession(): Error closing TTS: %v", err)
		}
		// [추가] LLM 세션 정리 요청
		llm.ClearSession(llmSessionID)
	}()

	/*
		// 상태 관리 (말하는 중에는 듣지 않음 - Half Duplex 유사 동작)
		var isListening = true
		var stateMutex sync.Mutex
		var lastFinalText string = ""
	*/

	// 3. 초기 인사말 처리 (LLM InitSession)
	go func() {
		log.Printf("orchestrateAudioSession(): Initializing LLM session...")
		initialUtterance, err := llm.InitSession(llmSessionID, scenarioKey, user.Profile, parentCtx)
		if err != nil {
			log.Printf("orchestrateAudioSession(): Failed to init LLM session: %v", err)
			return
		}

		log.Printf("orchestrateAudioSession(): LLM Init -> %s", initialUtterance)

		// TTS 변환 및 전송
		responseAudio, err := ttsClient.ConvertTextToAudio(initialUtterance)
		if err == nil {
			startTime := time.Since(sessionStartTime)
			archiveS2CChan <- archiver.ArchiveS2CJob{Data: responseAudio, StartTime: startTime}
			serverChan <- responseAudio
		} else {
			log.Printf("orchestrateAudioSession(): Failed to convert initial TTS: %v", err)
		}
	}()

	sttResultChan := make(chan string, 10)
	sttErrChan := make(chan error, 1)

	// 솔직히 난 왜 State Machine이 필요한지 모르겠음 ㅇㅇ

	// STT 수신 고루틴 시작
	go sttRecognizer.ReceiveTranslatedText(sttResultChan, sttErrChan)

	// 4. 메인 루프
	for {
		select {
		case <-parentCtx.Done():
			log.Printf("orchestrateAudioSession(): Context Canceled with %s", username)
			return

		// [오디오 수신] Client -> Server
		case audioChunk, ok := <-clientChan:
			if !ok {
				log.Printf("orchestrateAudioSession(): Client audio channel closed for: %s", username)
				return
			}

			// 무조건 아카이빙
			archiveC2SChan <- audioChunk

			/*
				// 상태에 따라 STT 전송 여부 결정
				stateMutex.Lock()
				currentListeningState := isListening
				stateMutex.Unlock()


				if currentListeningState {
					if err := sttRecognizer.SendAudio(audioChunk); err != nil {
						log.Printf("Failed to send audio to STT: %v", err)
					}
				}
			*/

		// [텍스트 수신] STT -> Logic
		case userText := <-sttResultChan:
			sttFinalTime := time.Since(sessionStartTime)
			cleanedText := strings.TrimSpace(userText)

			/*
				stateMutex.Lock()
				// 듣기 모드가 아니거나, 빈 텍스트거나, 중복된 텍스트면 무시
				if !isListening || cleanedText == "" || cleanedText == lastFinalText {
					stateMutex.Unlock()
					continue
				}

				// 유효한 입력이 들어오면 즉시 듣기 중단 (AI가 답변 준비)
				isListening = false
				lastFinalText = userText
				stateMutex.Unlock()

			*/
			log.Printf("orchestrateAudioSession(): STT [FINAL] -> %s", userText)

			// [변경] 별도 고루틴에서 LLM 호출 -> TTS -> 전송 수행
			go func(textInput string, sttTimestamp time.Duration) {
				// A. LLM Chat 호출
				log.Printf("orchestrateAudioSession(): Calling LLM for: %s", textInput)
				chatResp, err := llm.Chat(llmSessionID, textInput, parentCtx)

				if err != nil {
					log.Printf("orchestrateAudioSession(): LLM Chat Error: %v", err)
					// 에러 발생 시 다시 듣기 모드로 복구해야 함
					/*
						stateMutex.Lock()
						isListening = true
						stateMutex.Unlock()
					*/
					return
				}

				aiText := chatResp.Utterance
				log.Printf("orchestrateAudioSession(): LLM Response -> %s", aiText)

				// B. TTS 변환
				responseAudio, err := ttsClient.ConvertTextToAudio(aiText)
				if err != nil {
					log.Printf("orchestrateAudioSession(): TTS Error: %v", err)
				} else {
					// C. 전송 및 아카이빙
					archiveS2CChan <- archiver.ArchiveS2CJob{Data: responseAudio, StartTime: sttTimestamp}
					serverChan <- responseAudio
				}

				/*
					// D. 처리 완료 후 다시 듣기 모드 활성화
					stateMutex.Lock()
					isListening = true
					log.Printf("... (State change: NOW LISTENING)")
					stateMutex.Unlock()
				*/

			}(cleanedText, sttFinalTime)

		case err := <-sttErrChan:
			log.Printf("orchestrateAudioSession(): STT stream error: %v", err)
			return
		}
	}
}

// 테스트용 함수
/*
func orchestrateVoiceEchoTest(user models.User, sessionStartTime time.Time,
	clientChan <-chan []byte, serverChan chan<- []byte, archiveC2SChan chan<- []byte,
	archiveS2CChan chan<- archiver.ArchiveS2CJob, parentCtx context.Context) {

	username := user.Username
	log.Printf("orchestrateVoiceSession(): [STT->TTS Echo Test Mode] started for user: %s", username)

	// WritePump 및 Archiver(G4) 고루틴에 종료 신호 전송
	defer close(serverChan)
	defer close(archiveC2SChan)
	defer close(archiveS2CChan)

	sttRecognizer, err := llm.NewStreamingRecognizer(parentCtx)
	if err != nil {
		return
	}

	ttsClient, err := llm.NewTTSClient(parentCtx)
	if err != nil {
		return
	}

	defer func() {
		log.Printf("orchestrateVoiceSession(): Cleaning up resources for %s", username)
		if err := sttRecognizer.Close(); err != nil {
			log.Printf("orchestrateVoiceSession(): Error closing STT: %v", err)
		}
		if err := ttsClient.Close(); err != nil {
			log.Printf("orchestrateVoiceSession(): Error closing TTS: %v", err)
		}
	}()

	var isListening = true
	var stateMutex sync.Mutex
	var lastFinalText string = ""

	// 첫 인사말 전송을 위한 Goroutine (TTS / 아카이빙 / 전송)
	go func() {
		initialUtterance := "STT TTS 에코 테스트 모드입니다. 좀 돼라 망할."
		log.Printf("orchestrateVoiceSession(): Sending initial test message...")
		responseAudio, err := ttsClient.ConvertTextToAudio(initialUtterance)
		if err == nil {
			startTime := time.Since(sessionStartTime)

			archiveS2CChan <- archiver.ArchiveS2CJob{Data: responseAudio, StartTime: startTime}
			serverChan <- responseAudio
		} else {
			log.Printf("orchestrateVoiceSession(): Failed to convert initial TTS: %v", err)
		}
	}()

	sttResultChan := make(chan string, 10)
	sttErrChan := make(chan error, 1)

	// STT 응답 수신을 위한 별도 Goroutine 시작
	go sttRecognizer.ReceiveTranslatedText(sttResultChan, sttErrChan) // (ReceiveTranscribedText)

	// 메인 처리 루프 (State Machine)
	for {
		select {
		case <-parentCtx.Done():
			log.Printf("orchestrateVoiceSession(): Canceled with %s", username)
			return

		case audioChunk, ok := <-clientChan:
			if !ok {
				log.Printf("orchestrateVoiceSession(): client audio in channel closed for user: %s", username)
				return
			}

			archiveC2SChan <- audioChunk

			stateMutex.Lock()
			currentListeningState := isListening
			stateMutex.Unlock()

			if currentListeningState {
				if err := sttRecognizer.SendAudio(audioChunk); err != nil {
					log.Printf("Failed to send audio to STT: %v", err)
				}
			}

		case userText := <-sttResultChan:
			sttFinalTime := time.Since(sessionStartTime)
			cleanedText := strings.TrimSpace(userText)

			stateMutex.Lock()
			if !isListening || cleanedText == "" || cleanedText == lastFinalText {
				continue
			}

			isListening = false
			lastFinalText = userText
			stateMutex.Unlock()
			log.Printf("orchestrateVoiceSession(): STT [FINAL] -> %s", userText)
			log.Printf("... (State change: NOW RESPONDING. Discarding audio input)")

			// [수정] TTS 변환 및 전송을 *별도 Goroutine*에서 처리
			go func(textToSpeak string, sttTimestamp time.Duration) {
				log.Printf("orchestrateVoiceSession(): Calling TTS for: %s", textToSpeak)
				responseAudio, err := ttsClient.ConvertTextToAudio(textToSpeak)

				if err != nil {
					log.Printf("orchestrateVoiceSession(): Failed to convert chat TTS: %v", err)
				} else {
					archiveS2CChan <- archiver.ArchiveS2CJob{Data: responseAudio, StartTime: sttTimestamp} // S->C 아카이빙
					serverChan <- responseAudio                                                            // S->C 전송
				}

				stateMutex.Lock()
				isListening = true
				log.Printf("... (State change: NOW LISTENING)")
				stateMutex.Unlock()

			}(cleanedText, sttFinalTime)

		case err := <-sttErrChan:
			log.Printf("orchestrateVoiceSession(): STT stream error: %v", err)
			return
		}
	}
}

*/

func archiveAudioConversation(username string, archiver *archiver.Archiver, c2sIn <-chan []byte,
	s2cIn <-chan archiver.ArchiveS2CJob, ctx context.Context) {

	log.Printf("archiveAudioConversation(): started for user: %s", username)
	defer archiver.CloseBaseTrack()

	for {
		select {
		case <-ctx.Done():
			log.Printf("runArchivingLogic(): Canceled for %s", username)
			return
		case chunk := <-c2sIn:
			// Interdium 반복 재생, 테스트용
			/*
				if !ok {
					// archiver.WriteC2S(chunk)
					return
				}
			*/
			archiver.WriteC2S(chunk)
		case job, ok := <-s2cIn:
			if !ok {
				return
			}
			archiver.WriteS2C(job)
		}
	}

}
