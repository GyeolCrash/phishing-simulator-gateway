# **Gateway of PishingSimulator**

LLM을 이용한 실시간 음성 기반 피싱 시뮬레이션 훈련을 위한 백엔드 서버


## **1. 기술 스택 및 환경 (Technology Stack & Environment)**

* **언어**: Go (Golang) 1.25 이상
* **웹 프레임워크**: Gin (gin-gonic/gin)
* **API 문서화**: Swagger (swaggo/gin-swagger)
* **WebSocket**: gorilla/websocket
* **인증**: golang-jwt/jwt/v4
* **비밀번호 해싱**: golang.org/x/crypto/bcrypt
* **AI/ML 서비스**: 
    * Google Cloud Speech-to-Text (STT)
    * Google Cloud Text-to-Speech (TTS)
* **데이터베이스**: SQLite (modernc.org/sqlite)
* **기타**: Rate Limiting (gin-limit-by-key), Godotenv

## **2. 설치 및 실행 (Getting Started)**

### **2.1. 전제 조건**

* **Go 1.25 이상**
* **Git**
* **FFmpeg** (필수)
  * 통화 기록(오디오)의 병합 및 포맷 변환(.webm/.raw → .mp3)에 사용됨
  * 서버의 시스템 PATH에 `ffmpeg` 명령어가 등록되어야 함
* **Google Cloud 서비스 계정 키** (STT/TTS 사용 시 필요)

### **2.2. 설치 및 실행**

1. 저장소 클론:
```bash
git clone [https://github.com/GyeolCrash/PishingSimulator_SecurityProject.git](https://github.com/GyeolCrash/PishingSimulator_SecurityProject.git)
```

2. 의존성 설치:
```bash
go mod tidy
```


3. 서버 실행:
```bash  
go run cmd/api/main.go
```

### **2.3. 환경 변수 설정 **

* 루트 디렉토리에 .env 파일을 생성하여 JWT 시크릿 키를 설정할 수 있습니다. (설정하지 않으면 internal/auth/token.go의 기본 키가 사용됨)  
  JWT\_SECRET\_KEY="your\_very\_strong\_secret\_key"
* 일부 키는 생성하지 않으면 Fatal 발생


## **3\. Directory Structure **
```
PishingSimulator_SecurityProject/
├── cmd/api/
│   └── main.go                  [실행] 서버 진입점, 라우터 및 미들웨어 설정, Swagger 설정
├── docs/                        [문서] Swagger API 명세서 자동 생성 폴더
│   ├── docs.go
│   ├── swagger.json
│   └── swagger.yaml
├── internal/
│   ├── archiver/
│   │   └── archiver.go          [로직] 통화 기록 아카이빙 처리
│   ├── auth/
│   │   └── token.go             [로직] JWT 토큰 생성 및 검증
│   ├── handler/                 [핸들러] HTTP 및 WebSocket 요청 처리
│   │   ├── audio_connection.go
│   │   ├── audio_process.go
│   │   ├── text_connection.go
│   │   ├── user_handler.go      [핸들러] 회원가입, 로그인, 프로필 등 유저 관련 API
│   │   └── websocket_handler.go [핸들러] 시뮬레이션 웹소켓 연결 관리
│   ├── llm/                     [AI] LLM 및 음성 처리 클라이언트
│   │   ├── client.go
│   │   ├── stt.go               [Google Cloud] Speech-to-Text 처리
│   │   └── tts.go               [Google Cloud] Text-to-Speech 처리
│   ├── middleware/
│   │   ├── auth.go              [미들웨어] JWT 기반 인증 처리
│   │   └── access_code.go       [미들웨어] 접근 제어/초대 코드 검증
│   ├── models/                  [모델] 데이터 구조체 정의
│   │   ├── record.go
│   │   ├── scenario.go
│   │   └── user.go
│   └── storage/                 [DB] 데이터베이스 접근 계층
│       ├── database.go          [DB] DB 초기화 및 테이블 생성
│       ├── record_storage.go    [DB] 시뮬레이션 기록 CRUD
│       └── user_storage.go      [DB] 사용자 정보 CRUD
├── .gitignore
├── go.mod
├── go.sum
└── README.md                    (본 파일)
```

## 4. 주요 기능
* 회원 정보 관리
* Client, LLM 중계
* STT/TTS 서비스 연결
* 대화 기록 녹음과 아카이빙