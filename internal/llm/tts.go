/**
* Name: 			tts.go
* Description: 		TTS 서버 연결 및 스트리밍 처리
* Workflow: 		TTS Recognizer 생성, 텍스트 전송, 오디오 수신
 */

package llm

import (
	"context"
	"errors"
	"log"
	"os"

	"google.golang.org/api/option"

	texttospeech "cloud.google.com/go/texttospeech/apiv1"
	"cloud.google.com/go/texttospeech/apiv1/texttospeechpb"
)

// TTS 연결 정보
type TTSClient struct {
	client *texttospeech.Client
	ctx    context.Context
}

// TTS 클라이언트 초기화
func NewTTSClient(ctx context.Context) (*TTSClient, error) {
	credentialsFile := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	client, err := texttospeech.NewClient(ctx, option.WithCredentialsFile(credentialsFile))
	if err != nil {
		return nil, errors.New("NewTTSClient(): failed to create TTS client: " + err.Error())
	}
	return &TTSClient{
		client: client,
		ctx:    ctx,
	}, nil
}

// 텍스트를 오디오로 변환
func (t *TTSClient) ConvertTextToAudio(text string) ([]byte, error) {
	log.Printf("ConvertTextToAudio(): Converting text to audio : %s", text)
	req := &texttospeechpb.SynthesizeSpeechRequest{
		Input: &texttospeechpb.SynthesisInput{
			InputSource: &texttospeechpb.SynthesisInput_Text{Text: text},
		},
		Voice: &texttospeechpb.VoiceSelectionParams{
			LanguageCode: "ko-KR",
			SsmlGender:   texttospeechpb.SsmlVoiceGender_FEMALE,
			Name:         "ko-KR-Wavenet-A",
		},
		AudioConfig: &texttospeechpb.AudioConfig{
			AudioEncoding:   texttospeechpb.AudioEncoding_LINEAR16,
			SampleRateHertz: 16000,
			SpeakingRate:    1.25,
			Pitch:           -3.8,
			VolumeGainDb:    -1.4,
		},
	}

	resp, err := t.client.SynthesizeSpeech(t.ctx, req)
	if err != nil {
		log.Printf("ConvertTextToAudio(): SynthesizeSpeech failed: %v", err)
		return nil, err
	}

	// Notice: 현 방식은 문장 전체가 변환된 후 오디오를 리턴함
	// 실제 통화의 지연 시간을 줄이려면 StreamingSythesize API 사용해야 함

	log.Printf("ConvertTextToAudio(): SynthesizeSpeech succeeded, audio size: %d bytes", len(resp.AudioContent))
	return resp.AudioContent, nil
}

// TTS 클라이언트 종료
func (t *TTSClient) Close() error {
	if t.client != nil {
		return t.client.Close()
	}
	return nil
}
