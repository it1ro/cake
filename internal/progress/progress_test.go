package progress

import (
	"bytes"
	"strings"
	"testing"
)

func TestBar_NilWriter_NoopSafe(t *testing.T) {
	b := New(1000, nil)
	b.Inc()
	b.Inc()
	b.Finish()
	// паники быть не должно
}

func TestBar_BelowMinTotal_Noop(t *testing.T) {
	var buf bytes.Buffer
	b := New(10, &buf) // < MinTotal
	b.Inc()
	b.Inc()
	b.Finish()
	if buf.Len() != 0 {
		t.Errorf("total < MinTotal must not draw: %q", buf.String())
	}
}

func TestBar_FirstIncDraws(t *testing.T) {
	var buf bytes.Buffer
	b := New(1000, &buf)
	b.Inc()
	if !strings.Contains(buf.String(), "1/1000") {
		t.Errorf("первый Inc должен рисовать: %q", buf.String())
	}
}

func TestBar_RateLimit(t *testing.T) {
	var buf bytes.Buffer
	b := New(1000, &buf)
	b.Inc() // рисует (lastDraw — zero time)
	sizeAfterFirst := buf.Len()
	b.Inc() // слишком быстро — не рисует
	if buf.Len() != sizeAfterFirst {
		t.Errorf("второй Inc не должен рисовать: %q", buf.String())
	}
}

func TestBar_FinishIdempotent(t *testing.T) {
	var buf bytes.Buffer
	b := New(500, &buf)
	b.Inc()
	b.Finish()
	size := buf.Len()
	b.Finish() // no-op
	if buf.Len() != size {
		t.Errorf("Finish должен быть идемпотентен")
	}
}

func TestBar_FinishEndsWithCR(t *testing.T) {
	var buf bytes.Buffer
	b := New(500, &buf)
	b.Inc()
	b.Finish()
	if !strings.HasSuffix(buf.String(), "\r") {
		t.Errorf("Finish должен заканчиваться \\r: %q", buf.String())
	}
}
