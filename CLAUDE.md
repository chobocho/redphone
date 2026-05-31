# RedPhone — 빨간전화기(X-PopUp) Go 클론

## 무엇 / 왜
중앙 서버 없이 같은 LAN의 PC들이 서로를 자동 발견하고 쪽지·파일을 주고받는
무설치 단일 바이너리 메신저. UI는 로컬 웹. 추가로 파일을 로컬 URL로 공유한다.
WHY: 사내/가정 LAN에서 "USB로 옮기기"를 대체. 외부 클라우드·서버 의존 0.

## 기술 제약 (지킬 것)
- Go 1.22+. stdlib 우선. 외부 의존성은 coder/websocket "하나만".
  WHY: 배포 단순화 + 종료(close) 처리가 깔끔.
- 웹 자산은 //go:embed 로 내장 → 실행 파일 하나로 동작.
- 발견=UDP(17000), 그 외 HTTP(17080) + HTTPS(17443, 사용중이면 OS자동).
- 피어 IP는 UDP 패킷 src addr에서 취득(페이로드 신뢰 금지).
- 파일 저장 시 경로 탈출(../) 차단, 파일명 충돌 회피.
- **TLS 신뢰는 지문 핀닝.** self-signed 인증서 + HELLO의 `fp`로만 검증.
  표준 CA·hostname 검증은 InsecureSkipVerify로 끄고 VerifyConnection에서
  leaf cert SHA-256을 직접 비교. 지문 없는 피어로는 송신 자체 거부.
- **메시지·파일메타는 TLS, 파일 본문은 평문 HTTP** (의도된 분리).

## 동시성 / 종료
- root context 하나를 모든 goroutine이 공유.
- 종료 경로 3개(stdin "exit" / POST /api/shutdown / SIGINT) → 모두 cancel(ctx).
- 종료 시: BYE 브로드캐스트 → srv.Shutdown(3s) → ticker/hub 정지 → Wait → exit 0.
- WS hub는 mutex 대신 channel로 클라이언트 맵을 단일 goroutine이 소유.

## 작업 규율
- 이슈 1개 = 프롬프트 1개. Todo.md의 한 항목만 처리.
- TDD: 실패하는 테스트(RED) 먼저 → 구현(GREEN) → 리팩터.
- 모든 동시성 코드는 `go test -race` 통과가 DoD.
- 태스크 완료마다 Git 체크포인트: feat(scope): ... / test(scope): ...
- 주석은 "왜"를 적는다(무엇은 코드가 말한다).

## 현재 상태
- [x] Phase 0~9 (#1~#14) 전부 구현 완료.
- [x] **Phase 10 (보안):** 메시지 TLS + 파일메타 TLS + 지문 핀닝.
  Discovery 프로토콜 v2(fp/httpsPort) 도입, v1 패킷 거부.
  `go test ./...` / `go vet ./...` 통과.
- 알려진 한계: 같은 PC 2인스턴스는 일부 Windows 스택의 브로드캐스트 loopback
  미지원으로 상호 발견이 안 될 수 있음(README 검증 절 참조). 2대 머신 LAN은 정상.
- `-race`는 C 컴파일러(cgo) 필요 — 이 개발 머신엔 미설치, CI에서 수행.
- v2 후보: 멀티캐스트 발견, 공유/히스토리 영속화, 파일 본문 TLS 옵션,
  지문 핀의 디스크 영속화(TOFU 이후 변동 감지).

## Definition of Done (공통)
- 해당 패키지 단위 테스트 통과 + `go vet` 무경고.
- 동시성 관련이면 `-race` 통과.
- 공개 함수/타입에 의도(why) 주석.
- 수동 시나리오 1개를 README의 "검증" 절에 기록.
