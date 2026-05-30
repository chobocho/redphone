# RedPhone ☎

2000년대 사내 LAN 메신저 **X-PopUp(별명: 빨간전화기)**의 현대식 Go 클론.
중앙 서버 없이 같은 네트워크의 PC들이 서로를 자동 발견하고 쪽지·파일을 주고받으며,
파일을 로컬 URL로 즉시 공유한다. UI는 네이티브 트레이 대신 **로컬 웹 UI**.

- **서버리스 P2P** — 중앙 서버 0대. 모든 인스턴스가 곧 서버이자 클라이언트.
- **단일 실행 파일** — 웹 자산은 `embed.FS`로 바이너리에 내장(`redphone.exe` 하나).
- **의존성 최소화** — 표준 라이브러리 우선. 외부 의존성은 `coder/websocket` 하나.

## 빌드 / 실행

```sh
go build -o redphone.exe ./cmd/redphone
./redphone.exe                       # 기본: HTTP 17080, Discovery 17000, 브라우저 자동 오픈
```

### 플래그

| 플래그 | 기본값 | 설명 |
|---|---|---|
| `--name` | 호스트명 | 화면에 표시될 이름 |
| `--port` | `17080` | HTTP 포트(사용 중이면 OS 자동 폴백) |
| `--dport` | `17000` | Discovery UDP 포트 |
| `--open` | `true` | 기동 시 기본 브라우저 자동 오픈 |

## 종료 (3경로 → 1수렴)

- 콘솔에 `exit` 입력
- 웹 UI 우상단 **⏻ 종료** 버튼 (`POST /api/shutdown`)
- `Ctrl+C` (SIGINT/SIGTERM)

셋 다 동일한 graceful 경로로 수렴한다:
BYE 브로드캐스트 → `http.Shutdown(3s)` → ticker/WS hub 정지 → 전 goroutine 합류 → exit 0.

## 아키텍처

하나의 프로세스 안에서 goroutine들이 `context.Context` 하나를 공유한다.
종료 신호는 어디서 오든 그 context를 취소해 전부 정리한다.

```
Discovery(UDP) ─┐                        ┌─ HTTP Server (UI·API·inbox·/s)
                ├─ App Core (context) ───┤
stdin/signal ───┘   Peer Registry        ├─ WS Hub (channel-driven)
                                          └─ Share Store (token→file)
```

- `internal/app` — 배선·생명주기·graceful shutdown
- `internal/discovery` — UDP HELLO/BYE 인코딩·송수신
- `internal/peer` — Peer 레지스트리(upsert/expire/snapshot)
- `internal/web` — HTTP 핸들러 + WS hub + embed 자산
- `internal/message` — 쪽지 + 히스토리
- `internal/transfer` — 파일 안전 저장(경로탈출 차단·충돌회피·스트리밍)
- `internal/share` — 토큰 발급 + content-type 분기 서빙

## 검증 (Verification)

### 자동 테스트

```sh
go test ./...          # 전체 단위·통합 테스트
go vet ./...           # 정적 분석(무경고)
# go test -race ./...  # 동시성 검증 — C 컴파일러(gcc/clang) 필요(cgo)
```

> **`-race` 참고:** 레이스 디텍터는 cgo(=C 컴파일러)를 요구한다. C 컴파일러가
> 없는 환경에서는 `-race`가 빌드되지 않으므로, MinGW/TDM-GCC 설치 후 또는 CI에서
> 실행한다. WS Hub·Peer 레지스트리 등 동시성 코드는 "맵을 단일 goroutine이 소유"
> 또는 RWMutex로 설계해 레이스를 구조적으로 회피한다.

### 수동 스모크 시나리오

1. 빌드 후 한 인스턴스 기동: `./redphone.exe --name alice --open=false`
2. `http://localhost:17080` 접속 → UI 확인.
3. **URL 공유**: 사이드바 "파일 공유 링크 만들기"로 이미지/텍스트 업로드 →
   `/s/<token>` 링크 클릭 → 이미지는 인라인, 텍스트는 (이스케이프된) 미리보기.
4. **종료**: `exit` 입력 또는 ⏻ 종료 버튼 → 프로세스가 graceful 종료.

### LAN(2대 머신) 시나리오

서로 다른 두 PC에서 각각 실행하면 ≤6초 내 상호 발견되어 피어 목록에 표시된다.
쪽지(≤1초 도착)·파일(SHA-256 동일)·공유 링크를 주고받고, 한쪽 종료 시 ≤2초 내(BYE)
상대 목록에서 사라진다.

> **같은 PC 2인스턴스 발견에 대한 솔직한 한계:** Discovery는 명세대로 UDP
> 브로드캐스트(255.255.255.255)를 쓴다. 같은 포트(17000)를 `SO_REUSEADDR`로
> 공유하도록 했지만, **일부 Windows 네트워크 스택은 브로드캐스트를 같은 호스트의
> 다른 로컬 소켓으로 되돌려주지 않는다(loopback 미지원).** 이 경우 같은 PC의 두
> 인스턴스는 서로를 못 볼 수 있다. 쪽지·파일·공유는 정상 동작하며, **2대 머신
> 실제 LAN에서는 브로드캐스트가 유선으로 나가 정상 발견된다.** 같은 PC에서 발견까지
> 데모하려면 멀티캐스트 전환이 필요하며 v2 과제로 분리한다.

## 범위 한정 (v1)

URL 공유는 **LAN 한정·휘발성**(인스턴스 종료 시 소멸), 인증은 토큰 비밀성에 의존.
외부망 노출·영속화·쪽지 히스토리 영속화는 v2 과제로 분리한다.
