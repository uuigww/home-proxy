package deploy

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestProgressStepAndOK(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf, 3)

	p.Step("connect")
	p.OK("done")

	out := buf.String()
	if !strings.Contains(out, "Step 1/3") {
		t.Errorf("expected step counter, got: %q", out)
	}
	if !strings.Contains(out, "connect") {
		t.Errorf("expected step title, got: %q", out)
	}
	if !strings.Contains(out, OKMark) {
		t.Errorf("expected OK mark %q in output, got: %q", OKMark, out)
	}
	if !strings.Contains(out, "✓") {
		t.Errorf("expected check glyph, got: %q", out)
	}
}

func TestProgressFail(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf, 1)
	p.Step("upload")
	p.Fail(errors.New("boom"))

	out := buf.String()
	if !strings.Contains(out, FailMark) {
		t.Errorf("expected FailMark %q in output, got: %q", FailMark, out)
	}
	if !strings.Contains(out, "✗") {
		t.Errorf("expected cross glyph, got: %q", out)
	}
	if !strings.Contains(out, "boom") {
		t.Errorf("expected error message, got: %q", out)
	}
}

func TestProgressNilSafe(t *testing.T) {
	var p *Progress
	// none of these should panic.
	p.Step("x")
	p.OK("y")
	p.Fail(nil)
	p.Info("z")
}

func TestProgressInfo(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf, 1)
	p.Info("hello")
	if !strings.Contains(buf.String(), "hello") {
		t.Errorf("expected info text, got: %q", buf.String())
	}
}

func TestStyleMarkersRenderGlyphs(t *testing.T) {
	if !strings.Contains(StyleOK.Render(OKMark), "✓") {
		t.Error("StyleOK must contain ✓")
	}
	if !strings.Contains(StyleFail.Render(FailMark), "✗") {
		t.Error("StyleFail must contain ✗")
	}
}
