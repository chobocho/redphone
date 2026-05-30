# RedPhone Todo

> 규칙: 한 번에 이슈 1개. TDD(RED→GREEN→리팩터). 끝나면 Git 체크포인트.

## Phase 0 — 스캐폴딩 / 생명주기
- [ ] #1 go.mod, cmd/redphone, internal/app 골격. context + signal + stdin "exit"
      로 즉시 종료되는 빈 앱. 테스트: cancel 시 Run이 ≤100ms 내 반환.

## Phase 1 — Peer 레지스트리
- [ ] #2 peer.Registry: Upsert/Snapshot/Expire(ttl). 단위 테스트로 만료 검증.

## Phase 2 — Discovery (UDP)
- [ ] #3 HELLO/BYE JSON encode/decode 테스트.
- [ ] #4 broadcast(ticker) + listen. src addr에서 IP 취득. 자기 id 필터.

## Phase 3 — HTTP + WS Hub
- [ ] #5 net/http 서버 + embed 정적 서빙 + 포트 폴백. /api/peers 핸들러 테스트.
- [ ] #6 channel 기반 WS Hub(register/unregister/broadcast). -race 통과.

## Phase 4 — 메시징
- [ ] #7 /api/send → 상대 /inbox/message 라운드트립 테스트. 수신 시 WS 푸시.

## Phase 5 — 파일 전송
- [ ] #8 /api/sendfile → /inbox/file 스트리밍 저장. SHA-256 왕복 동일 + ../ 차단.

## Phase 6 — URL 공유 (신규)
- [ ] #9 share.Store: 토큰 발급/조회/회수 테스트.
- [ ] #10 GET /s/{token}: image=인라인, text=미리보기, 그 외=다운로드. 404 테스트.

## Phase 7 — 웹 UI
- [ ] #11 index.html/app.js/style.css: 피어목록·대화·첨부·공유·종료버튼. 반응형(Fold).
- [ ] #12 브라우저 자동 오픈(OS 분기).

## Phase 8 — Graceful Shutdown 통합
- [ ] #13 POST /api/shutdown + BYE 브로드캐스트 + Shutdown(3s). 두 경로 모두 -race.

## Phase 9 — 다듬기
- [ ] #14 구조화 로깅, 설정 플래그(--name --port), README 검증 절, 동시성 단순화.
