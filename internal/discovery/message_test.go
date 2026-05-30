package discovery

import "testing"

func TestHelloRoundTrip(t *testing.T) {
	in := Hello("uuid-1", "chobocho", 17080, 1_730_000_000_000)
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

func TestDecodeRejectsBadVersion(t *testing.T) {
	// WHY: 프로토콜 버전이 다른 미래 클라이언트의 패킷은 조용히 거른다.
	if _, err := Decode([]byte(`{"v":2,"type":"hello","id":"x"}`)); err == nil {
		t.Fatal("expected error for version mismatch")
	}
}

func TestDecodeRejectsUnknownType(t *testing.T) {
	if _, err := Decode([]byte(`{"v":1,"type":"ping","id":"x"}`)); err == nil {
		t.Fatal("expected error for unknown type")
	}
}

func TestDecodeRejectsEmptyID(t *testing.T) {
	// WHY: id가 없으면 자기-필터링/중복제거가 불가능하므로 폐기.
	if _, err := Decode([]byte(`{"v":1,"type":"hello","id":""}`)); err == nil {
		t.Fatal("expected error for empty id")
	}
}

func TestDecodeRejectsMalformedJSON(t *testing.T) {
	if _, err := Decode([]byte(`{not json`)); err == nil {
		t.Fatal("expected error for malformed json")
	}
}
