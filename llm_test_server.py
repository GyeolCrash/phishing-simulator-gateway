from fastapi import FastAPI, HTTPException, Request, status
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse
import json
from pydantic import BaseModel
from typing import Optional, Dict, List
from openai import OpenAI

# === 설정 ===
app = FastAPI()
client = OpenAI(api_key="tDCI8PKwNqvTDXxzQmrcg7pE63Jx0Xr3",
                base_url="https://factchat-cloud.mindlogic.ai/v1/api/openai"
) 

# 세션 저장소 (메모리 DB) - 실제 서비스에선 Redis 권장
session_storage: Dict[str, List[Dict[str, str]]] = {}
session_metadata: Dict[str, Dict] = {}

# === 데이터 모델 (Go client.go 구조와 일치) ===
class UserInfo(BaseModel):
    name: str
    age: int
    gender: str

class InitRequest(BaseModel):
    session_id: str
    scenario: str
    user_info: UserInfo
    temperature: float

class InitResponse(BaseModel):
    utterance: str

class ChatRequest(BaseModel):
    session_id: str
    user_text: str

class ChatResponse(BaseModel):
    utterance: str
    next_step: str = "continue"  # 대화 진행 상태

class ControlRequest(BaseModel):
    session_id: Optional[str] = None
    clear_session: bool

# === 시나리오 프롬프트 관리 ===
SCENARIOS = {
    "loan_scam": {
        "system": "당신은 '김민수 대리'라는 이름의 저금리 대출 상담원입니다. 사용자를 속여 기존 대출금을 상환하게 하거나 보증금을 입금하게 유도해야 합니다. 말은 짧고 단호하게 하세요. 절대 당신이 AI라고 밝히지 마세요.",
        "first_msg": "여보세요? {name} 고객님 맞으십니까? 이번에 정부 지원 저금리 대출 대상자로 선정되셔서 연락드렸습니다."
    },
    "police_impersonation": {
        "system": "당신은 서울지방경찰청 사이버수사대 수사관입니다. 사용자의 명의가 대포통장에 도용되었다며 위협하고, 수사에 협조하지 않으면 구속된다고 압박하세요. 강압적인 어조를 사용하세요.",
        "first_msg": "서울지방경찰청 지능범죄수사대입니다. {name} 본인 맞으십니까? 지금 본인 명의로 된 대포통장이 범죄에 연루되었습니다."
    },
    "family_emergency": {
        "system": "당신은 사용자의 가족을 납치했거나 사고가 났다고 주장하는 사람입니다. 당장 합의금을 보내지 않으면 가족에게 해를 가하겠다고 협박하세요. 매우 다급하고 거친 말투를 쓰세요.",
        "first_msg": "아저씨, 지금 당신 아들이 내 차를 긁고 튀었는데, 이거 어떻게 할 거야? 당장 합의 안 보면 경찰 부른다."
    }
}

# === 엔드포인트 구현 ===

@app.post("/session/init", response_model=InitResponse)
async def init_session(req: InitRequest):
    
    print(f"[Init] User: {req.user_info.name}, Scenario: {req.scenario}")
    scenario_data = SCENARIOS.get(req.scenario)
    
    # 시나리오가 없을 경우 기본값 설정
    if not scenario_data:
        scenario_data = SCENARIOS["loan_scam"]

    # 시스템 프롬프트 구성 (TTS를 고려하여 짧은 문장 유도)
    system_prompt = (
        f"{scenario_data['system']} "
        f"상대방의 이름은 {req.user_info.name}입니다. "
        "응답은 구어체로 하고, 한 번에 2~3문장 이내로 짧게 말하세요(TTS용)."
    )

    # 세션 저장
    session_storage[req.session_id] = [
        {"role": "system", "content": system_prompt}
    ]
    session_metadata[req.session_id] = {
        "scenario": req.scenario,
        "user_info": req.user_info
    }

    # 첫 마디 생성 (미리 정의된 메시지 + 개인화)
    first_utterance = scenario_data['first_msg'].format(name=req.user_info.name)
    
    # 대화 기록에 추가
    session_storage[req.session_id].append({"role": "assistant", "content": first_utterance})

    print(f"[Session Init] ID: {req.session_id}, Scenario: {req.scenario}")
    return InitResponse(utterance=first_utterance)


@app.post("/chat", response_model=ChatResponse)
async def chat_process(req: ChatRequest):
    """
    사용자의 텍스트 입력을 받아 LLM 응답을 반환합니다.
    """
    history = session_storage.get(req.session_id)
    
    if not history:
        raise HTTPException(status_code=404, detail="Session not found")

    # 사용자 입력 추가
    history.append({"role": "user", "content": req.user_text})

    try:
        # OpenAI API 호출 (gpt-4o-mini 사용 권장)
        response = client.chat.completions.create(
            model="gpt-4o-mini",
            messages=history,
            temperature=0.8, # 시뮬레이션을 위해 약간의 창의성 허용
            max_tokens=150   # 답변이 너무 길어지지 않도록 제한
        )
        
        ai_utterance = response.choices[0].message.content
        
        # AI 응답 추가
        history.append({"role": "assistant", "content": ai_utterance})
        
        # (심화) 대화가 종료되었는지 판단하는 로직을 추가할 수 있음
        # 여기서는 단순 continue로 설정
        next_step = "continue" 

        print(f"[Chat] Session: {req.session_id} | User: {req.user_text} | AI: {ai_utterance}")
        return ChatResponse(utterance=ai_utterance, next_step=next_step)

    except Exception as e:
        print(f"Error during LLM call: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/session/control")
async def control_session(req: ControlRequest):
    """
    세션을 종료하거나 정리합니다.
    """
    if req.clear_session and req.session_id:
        if req.session_id in session_storage:
            del session_storage[req.session_id]
        if req.session_id in session_metadata:
            del session_metadata[req.session_id]
        print(f"[Session Cleared] ID: {req.session_id}")
    
    return {"status": "success"}

@app.exception_handler(RequestValidationError)
async def validation_exception_handler(request: Request, exc: RequestValidationError):
    # 에러의 상세 내용을 로그에 출력합니다.
    error_details = exc.errors()
    print(f"\n[ERROR] Validation Failed: {json.dumps(error_details, indent=2, ensure_ascii=False)}\n")
    
    # 클라이언트가 보낸 원본 Body도 확인해봅니다.
    body = await request.body()
    print(f"[ERROR] Received Body: {body.decode('utf-8')}\n")

    return JSONResponse(
        status_code=status.HTTP_422_UNPROCESSABLE_ENTITY,
        content={"detail": error_details, "body": body.decode('utf-8')},
    )

# 서버 실행: uvicorn server:app --host 0.0.0.0 --port 8001 --reload