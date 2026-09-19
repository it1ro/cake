package clipboard

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

func TestEncode(t *testing.T) {
	data := []byte("hello, cake")
	got := Encode(data)
	want := "\x1b]52;c;" + base64.StdEncoding.EncodeToString(data) + "\x07"
	if got != want {
		t.Errorf("Encode mismatch\n want: %q\n  got: %q", want, got)
	}
}

func TestEncode_RoundTrip(t *testing.T) {
	data := []byte("package main\n\nfunc main() {}\n")
	seq := Encode(data)

	// вытащим base64 между "c;" и BEL
	i := strings.Index(seq, "c;")
	j := strings.IndexByte(seq, '\x07')
	if i < 0 || j < 0 || j <= i+2 {
		t.Fatalf("malformed sequence: %q", seq)
	}
	dec, err := base64.StdEncoding.DecodeString(seq[i+2 : j])
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !bytes.Equal(dec, data) {
		t.Errorf("round-trip mismatch")
	}
}

func TestCopy(t *testing.T) {
	var buf bytes.Buffer
	if err := Copy(&buf, []byte("x")); err != nil {
		t.Fatal(err)
	}
	if buf.String() != Encode([]byte("x")) {
		t.Errorf("Copy wrote %q, want %q", buf.String(), Encode([]byte("x")))
	}
}
