# RedPhone Todo

## 완료

- [x] Phase 0: 앱 생명주기, 종료 경로, 기본 진입점
- [x] Phase 1: 피어 레지스트리
- [x] Phase 2: UDP HELLO/BYE discovery
- [x] Phase 3: HTTP 서버, 정적 UI, WebSocket hub
- [x] Phase 4: 1:1 메시지 송수신
- [x] Phase 5: 파일 전송
- [x] Phase 6: URL 공유 링크
- [x] Phase 7: 웹 UI
- [x] Phase 8: graceful shutdown
- [x] Phase 9: 문서화 및 검증 정리
- [x] 친구 IP 직접 추가/수정/삭제, 전체 스캔
- [x] 전체 브로드캐스트 메시지
- [x] SQLite 기반 채팅 히스토리 영속 저장
- [x] 대화별 히스토리 삭제
- [x] 채팅 URL 자동 링크 처리
- [x] `alert`/`confirm`/`prompt` 제거 후 커스텀 모달 적용
- [x] 다크/화이트 테마
- [x] 화이트 테마 가독성 수정

## 현재 상태

- 메시지 히스토리는 `redphone.db`에 저장
- 친구 IP 목록은 `peers.json`에 저장
- `*.db`, `peers.json`, `downloads/`는 `.gitignore` 대상

## 남은 후보 작업

- [ ] 수신 파일 이벤트에도 송신 피어 정보를 포함해서 대화별 파일 로그 정교화
- [ ] 채팅 검색
- [ ] 히스토리 export/import
- [ ] 대화방별 unread 영속화
- [ ] 모바일 UI 미세 조정
- [ ] E2E 브라우저 테스트 추가
