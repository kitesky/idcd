package service

import "testing"

func TestRenderPDF_HasMagicHeader(t *testing.T) {
	out, err := renderPDF(&Order{ID: "vo_x"}, nil, nil)
	if err != nil {
		t.Fatalf("renderPDF: %v", err)
	}
	if len(out) < 5 || string(out[:5]) != "%PDF-" {
		t.Fatalf("renderPDF output missing %%PDF- header: %q", out[:minInt(5, len(out))])
	}
}
