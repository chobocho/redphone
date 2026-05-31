package web

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chobocho/redphone/internal/peer"
)

// fakeTargets is an in-memory PeerControl for handler tests.
type fakeTargets struct {
	ips     []string
	scanN   int
	scanErr error
}

func (f *fakeTargets) AddTarget(ip string) error {
	if net.ParseIP(ip) == nil {
		return errors.New("invalid ip")
	}
	for _, e := range f.ips {
		if e == ip {
			return nil
		}
	}
	f.ips = append(f.ips, ip)
	return nil
}
func (f *fakeTargets) RemoveTarget(ip string) {
	out := f.ips[:0]
	for _, e := range f.ips {
		if e != ip {
			out = append(out, e)
		}
	}
	f.ips = out
}
func (f *fakeTargets) Targets() []string     { return f.ips }
func (f *fakeTargets) ScanLAN() (int, error) { return f.scanN, f.scanErr }

func targetServer(ft *fakeTargets) *Server {
	return New(Options{Reg: peer.NewRegistry(), SelfID: "self", Name: "tester", Targets: ft})
}

func doReq(s *Server, method, path, body string) *httptest.ResponseRecorder {
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, r)
	return rec
}

// TestAddTargetValid는 유효 IP 등록이 200과 갱신된 목록을 주는지 본다.
func TestAddTargetValid(t *testing.T) {
	ft := &fakeTargets{}
	rec := doReq(targetServer(ft), http.MethodPost, "/api/targets", `{"ip":"192.168.0.5"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body)
	}
	var got struct{ Targets []string }
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if len(got.Targets) != 1 || got.Targets[0] != "192.168.0.5" {
		t.Errorf("targets = %v, want [192.168.0.5]", got.Targets)
	}
}

// TestAddTargetInvalid는 잘못된 IP가 400으로 거부되는지 본다.
func TestAddTargetInvalid(t *testing.T) {
	rec := doReq(targetServer(&fakeTargets{}), http.MethodPost, "/api/targets", `{"ip":"not-an-ip"}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// TestListTargets는 빈 목록이 null이 아닌 []로 직렬화되는지 본다.
func TestListTargets(t *testing.T) {
	rec := doReq(targetServer(&fakeTargets{}), http.MethodGet, "/api/targets", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"targets":[]`) {
		t.Errorf("body = %s, want empty array", rec.Body.String())
	}
}

// TestRemoveTarget는 DELETE가 해당 IP를 지우는지 본다.
func TestRemoveTarget(t *testing.T) {
	ft := &fakeTargets{ips: []string{"192.168.0.5", "192.168.0.9"}}
	rec := doReq(targetServer(ft), http.MethodDelete, "/api/targets/192.168.0.5", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if len(ft.ips) != 1 || ft.ips[0] != "192.168.0.9" {
		t.Errorf("ips = %v, want [192.168.0.9]", ft.ips)
	}
}

// TestEditTarget는 PUT가 옛 IP를 새 IP로 교체하는지 본다.
func TestEditTarget(t *testing.T) {
	ft := &fakeTargets{ips: []string{"192.168.0.5"}}
	rec := doReq(targetServer(ft), http.MethodPut, "/api/targets", `{"old":"192.168.0.5","new":"192.168.0.6"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body)
	}
	if len(ft.ips) != 1 || ft.ips[0] != "192.168.0.6" {
		t.Errorf("ips = %v, want [192.168.0.6]", ft.ips)
	}
}

// TestEditTargetInvalidKeepsOld는 새 IP가 잘못되면 옛 IP가 보존되는지 본다.
func TestEditTargetInvalidKeepsOld(t *testing.T) {
	ft := &fakeTargets{ips: []string{"192.168.0.5"}}
	rec := doReq(targetServer(ft), http.MethodPut, "/api/targets", `{"old":"192.168.0.5","new":"bad"}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	if len(ft.ips) != 1 || ft.ips[0] != "192.168.0.5" {
		t.Errorf("ips = %v, want unchanged [192.168.0.5]", ft.ips)
	}
}

// TestScan는 스캔 트리거가 보낸 probe 수를 반환하는지 본다.
func TestScan(t *testing.T) {
	rec := doReq(targetServer(&fakeTargets{scanN: 42}), http.MethodPost, "/api/scan", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"sent":42`) {
		t.Errorf("body = %s, want sent:42", rec.Body.String())
	}
}

// TestSelfReportsName는 /api/self가 이름(과 IP 키)을 돌려주는지 본다.
// IP는 환경에 따라 빈 값일 수 있으므로 이름과 200만 단정한다.
func TestSelfReportsName(t *testing.T) {
	rec := doReq(targetServer(&fakeTargets{}), http.MethodGet, "/api/self", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got struct {
		ID, Name, IP string
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("bad json: %v (%s)", err, rec.Body)
	}
	if got.Name != "tester" {
		t.Errorf("name = %q, want tester", got.Name)
	}
}

// TestScanNotRunning는 미기동 스캔 에러가 409로 매핑되는지 본다.
func TestScanNotRunning(t *testing.T) {
	rec := doReq(targetServer(&fakeTargets{scanErr: errors.New("not running")}), http.MethodPost, "/api/scan", "")
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
}
