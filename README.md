# RedPhone

같은 LAN 안에서 동작하는 단일 바이너리 P2P 메신저입니다. 중앙 서버 없이 피어를 자동 발견하고, 메시지/파일/공유 링크를 주고받습니다.

현재 버전 기준 핵심 기능:

- 피어 자동 발견: UDP HELLO/BYE
- 1:1 메시지, 전체 브로드캐스트
- 파일 전송, URL 공유 링크
- 수신 파일 열기 링크 + 파일 전송 소요 시간/평균 속도 표시
- 채팅 히스토리 SQLite 영속 저장
- 대화별 삭제
- 채팅 본문 URL 자동 링크 처리
- 커스텀 모달 UI
- 다크/화이트 테마

## 빌드

```sh
go build -o redphone.exe ./cmd/redphone
```

## 실행

```sh
./redphone.exe
```

기본값:

- HTTP: `17080`
- HTTPS: `17443`
- Discovery UDP: `17000`
- 채팅 DB: `redphone.db`

## 플래그

| 플래그 | 기본값 | 설명 |
|---|---:|---|
| `--name` | 호스트명 | 화면 표시 이름 |
| `--port` | `17080` | HTTP 포트 |
| `--sport` | `17443` | HTTPS 포트 |
| `--dport` | `17000` | Discovery UDP 포트 |
| `--db` | `redphone.db` | SQLite 히스토리 DB 경로 |
| `--open` | `true` | 실행 시 브라우저 자동 열기 |
| `--scan` | `false` | 서브넷 전체 HELLO 스캔 |

## 히스토리 저장

채팅 히스토리는 SQLite에 저장됩니다.

- 기본 파일: `redphone.db`
- 브라우저를 닫아도 기록 유지
- Termux/cgo 제약을 피하려고 순수 Go SQLite 드라이버(`modernc.org/sqlite`) 사용
- `.gitignore`에서 `*.db`를 제외하도록 되어 있어 기본 DB가 저장소에 들어가지 않음

## UI 기능

### 파일 전송 로그

- 파일을 받으면 대화에 열기 링크가 함께 저장됩니다.
- 파일 메시지에는 소요 시간과 평균 속도가 같이 표시됩니다.

### 대화 삭제

- 현재 선택한 피어 또는 전체 대화를 삭제할 수 있습니다.
- 삭제는 로컬 SQLite 히스토리에서 바로 반영됩니다.

### 링크 클릭

메시지에 아래 형태의 URL이 오면 자동으로 링크로 렌더링됩니다.

```text
http://192.168.45.73:17080/s/...
https://example.com/...
```

### 팝업

기존 `alert` / `confirm` / `prompt` 대신 커스텀 모달을 사용합니다.

- 종료 확인
- 친구 IP 수정
- 친구 IP 삭제
- 대화 삭제

### 테마

- 다크 테마
- 화이트 테마

화이트 테마는 별도 색상 토큰을 사용해 헤더/입력 영역/오버레이까지 같이 전환됩니다.

## 친구 IP 직접 추가

브로드캐스트가 막힌 환경에서는 사이드바에서 친구 IP를 직접 등록할 수 있습니다.

- 추가: `POST /api/targets`
- 수정: `PUT /api/targets`
- 삭제: `DELETE /api/targets/{ip}`
- 조회: `GET /api/targets`
- 전체 스캔: `POST /api/scan`

등록 목록은 `peers.json`에 저장됩니다.

## 종료 경로

다음 셋 중 하나로 종료할 수 있습니다.

- 콘솔에 `exit`
- UI의 종료 버튼
- `Ctrl+C`

종료 시 graceful shutdown 경로로 정리됩니다.

## 보안 모델

- 메시지와 파일 메타데이터는 HTTPS
- 파일 본문은 1회용 토큰 기반 PUT
- 피어 간 HTTPS는 self-signed 인증서 fingerprint pinning 사용
- fingerprint가 없는 피어와는 TLS 메시지 송신 거부

## 패키지 구성

- `cmd/redphone`: 진입점
- `internal/app`: 프로세스 생명주기
- `internal/discovery`: UDP 발견
- `internal/peer`: 피어 레지스트리
- `internal/peerbook`: `peers.json` 영속 저장
- `internal/message`: SQLite 히스토리 저장소
- `internal/web`: HTTP/HTTPS/WebSocket/UI
- `internal/share`: 공유 링크 저장소
- `internal/transfer`: 파일 저장
- `internal/tlsid`: TLS identity / fingerprint pinning

## 테스트

```sh
go test ./...
```

## 검증

- A에서 B로 파일을 전송하면 B 대화창에 `열기:` 링크가 포함된 파일 메시지가 생기고, 링크 클릭 시 받은 파일이 열리거나 다운로드됩니다.
- 같은 파일 메시지에 `소요 시간`과 `평균 속도`가 함께 표시됩니다.

## 현재 범위

현재 히스토리는 로컬 장치 단위 저장입니다. 서로 다른 기기 간 채팅 히스토리를 동기화하지는 않습니다.
