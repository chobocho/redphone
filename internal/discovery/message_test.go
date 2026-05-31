package discovery

import "testing"

func TestHelloRoundTrip(t *testing.T) {
	in := Hello("uuid-1", "chobocho", 17080, 17081, "abcd1234", 1_730_000_000_000)
	b, err := in.Encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	out, err := Decode(b)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out != in {
		t.Fatalf("round-trip mismatch:\n in=%+v\nout=%+v", in, out)
	}
	if out.Type != TypeHello || out.V != Version {
		t.Fatalf("unexpected header: %+v", out)
	}
	if out.FP != "abcd1234" || out.HTTPSPort != 17081 {
		t.Fatalf("v2 fields lost: %+v", out)
	}
}

func TestByeRoundTrip(t *testing.T) {
	in := Bye("uuid-1")
	b, err := in.Encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	out, err := Decode(b)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Type != TypeBye || out.ID != "uuid-1" {
		t.Fatalf("bye mismatch: %+v", out)
	}
}

func TestDecodeRejectsOldVersion(t *testing.T) {
	// WHY: v1은 fp/httpsPort가 없어 TLS 채널을 못 연다 → 조용히 거른다.
	if _, err := Decode([]byte(`{"v":1,"type":"hello","id":"x","fp":"abc"}`)); err == nil {
		t.Fatal("expected error for v1 packet")
	}
}

func TestDecodeRejectsFutureVersion(t *testing.T) {
	if _, err := Decode([]byte(`{"v":99,"type":"hello","id":"x"}`)); err == nil {
		t.Fatal("expected error for future version")
	}
}

func TestDecodeRejectsUnknownType(t *testing.T) {
	if _, err := Decode([]byte(`{"v":2,"type":"ping","id":"x","fp":"abc"}`)); err == nil {
		t.Fatal("expected error for unknown type")
	}
}

func TestDecodeRejectsEmptyID(t *testing.T) {
	// WHY: id가 없으면 자기-필터링/중복제거가 불가능하므로 폐기.
	if _, err := Decode([]byte(`{"v":2,"type":"hello","id":"","fp":"abc"}`)); err == nil {
		t.Fatal("expected error for empty id")
	}
}

func TestDecodeRejectsHelloWithoutFingerprint(t *testing.T) {
	// WHY: 지문 없는 HELLO는 TLS 핀닝이 불가능 → 송신 대상으로 쓸 수 없다.
	if _, err := Decode([]byte(`{"v":2,"type":"hello","id":"x"}`)); err == nil {
		t.Fatal("expected error for missing fingerprint")
	}
}

// BYE에는 지문이 필요 없다(제거 통지일 뿐 신뢰 결정에 안 쓰임).
func TestDecodeAcceptsByeWithoutFingerprint(t *testing.T) {
	if _, err := Decode([]byte(`{"v":2,"type":"bye","id":"x"}`)); err != nil {
		t.Fatalf("bye without fp should be accepted: %v", err)
	}
}

func TestDecodeRejectsMalformedJSON(t *testing.T) {
	if _, err := Decode([]byte(`{not json`)); err == nil {
		t.Fatal("expected error for malformed json")
	}
}
