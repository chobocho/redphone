// Package peerbook persists the manually-added friend-IP list to disk.
//
// WHY: 브로드캐스트가 막힌 망에서는 친구 IP를 직접 등록해 발견한다. 그 목록을
// 실행 파일과 같은 폴더의 peers.json에 저장해 재시작해도 다시 입력할 필요가
// 없게 한다. 발견(discovery)은 순수 네트워킹만 책임지므로 파일 IO는 여기 둔다.
package peerbook

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// fileName is the on-disk store, kept next to the binary by DefaultPath.
const fileName = "peers.json"

// book is the JSON shape. 배열을 객체로 감싸 향후 필드 확장(라벨 등) 여지를 둔다.
type book struct {
	Targets []string `json:"targets"`
}

// DefaultPath returns peers.json in the executable's directory, falling back to
// the current working directory when the executable path can't be resolved
// (예: 일부 테스트/샌드박스 환경).
func DefaultPath() string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), fileName)
	}
	return fileName
}

// Load reads the saved target IPs. 파일이 없으면 빈 목록을 반환한다(에러 아님) —
// 최초 실행은 정상 상황이다.
func Load(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var bk book
	if err := json.Unmarshal(b, &bk); err != nil {
		return nil, err
	}
	return bk.Targets, nil
}

// Save atomically writes the target IPs (temp file + rename) so a crash mid-write
// never leaves a truncated peers.json. 목록은 정렬해 디프 노이즈를 줄인다.
func Save(path string, ips []string) error {
	out := append([]string(nil), ips...)
	sort.Strings(out)
	b, err := json.MarshalIndent(book{Targets: out}, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
