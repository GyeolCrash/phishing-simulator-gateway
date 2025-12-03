/**
* Name: 			stt.go
* Description: 		STT 서버 연결 및 스트리밍 처리
* Workflow: 		STT Recognizer 생성, 오디오 전송, 텍스트 수신
 */

package llm

import (
	"context"
	"errors"
	"io"
	"log"
	"os"

	speech "cloud.google.com/go/speech/apiv1"
	speechpb "cloud.google.com/go/speech/apiv1/speechpb"
	"google.golang.org/api/option"
)

type StreamingRecognizer struct {
	stream speechpb.Speech_StreamingRecognizeClient
	ctx    context.Context
}

// STT Recognizer 초기화
func NewStreamingRecognizer(ctx context.Context) (*StreamingRecognizer, error) {
	credentialsFile := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	if credentialsFile == "" {
		return nil, errors.New("NewStreamingRecognizer(): GOOGLE_APPLICATION_CREDENTIALS environment variable is not set")
	}

	client, err := speech.NewClient(ctx, option.WithCredentialsFile(os.Getenv(credentialsFile)))
	if err != nil {
		log.Printf("NewStreamingRecognizer(): failed to create speech client: %v", err)
		return nil, err
	}

	stream, err := client.StreamingRecognize(ctx)
	if err != nil {
		log.Printf("NewStreamingRecognizer(): failed to create streaming recognize client: %v", err)
		return nil, err
	}

	config := &speechpb.StreamingRecognitionConfig{
		Config: &speechpb.RecognitionConfig{
			Encoding:          speechpb.RecognitionConfig_WEBM_OPUS,
			SampleRateHertz:   16000,
			AudioChannelCount: 1,
			LanguageCode:      "ko-KR",
		},
		InterimResults:  true,
		SingleUtterance: false,
	}
	if err := stream.Send(&speechpb.StreamingRecognizeRequest{
		StreamingRequest: &speechpb.StreamingRecognizeRequest_StreamingConfig{
			StreamingConfig: config,
		},
	}); err != nil {
		log.Printf("NewStreamingRecognizer(): failed to send initial config: %v", err)
		return nil, err
	}

	return &StreamingRecognizer{
		stream: stream,
		ctx:    ctx,
	}, nil
}

// gRPC 스트리밍 오디오 전송
func (r *StreamingRecognizer) SendAudio(audioData []byte) error {
	return r.stream.Send(&speechpb.StreamingRecognizeRequest{
		StreamingRequest: &speechpb.StreamingRecognizeRequest_AudioContent{
			AudioContent: audioData,
		},
	})
}

// gRPC 스트리밍 응답 수신
func (r *StreamingRecognizer) ReceiveTranslatedText(resultChannel chan<- string, errChan chan<- error) {
	//log.Printf("ReceiveTranslatedText(): started")
	for {
		resp, err := r.stream.Recv()
		if err == io.EOF {
			log.Printf("ReceiveTranslatedText(): stream closed by server")
			return
		}
		if err != nil {
			log.Printf("ReceiveTranslatedText(): error receiving response: %v", err)
			return
		}

		if err := resp.Error; err != nil {
			log.Printf("ReceiveTranslatedText(): received error from server: %v", err)
			errChan <- errors.New(err.Message)
			continue
		}

		for _, result := range resp.Results {
			if result.IsFinal {
				log.Printf("ReceiveTranslatedText(): final result: %s", result.Alternatives[0].Transcript)
				resultChannel <- result.Alternatives[0].Transcript
			} else {
				//log.Printf("ReceiveTranslatedText(): interim result: %s", result.Alternatives[0].Transcript)
				// resultChannel <- result.Alternatives[0].Transcript
			}
		}
	}
}

// gRPC 스트리밍 종료
func (r *StreamingRecognizer) Close() error {
	if r.stream == nil {
		return r.stream.CloseSend()
	}
	return nil
}
