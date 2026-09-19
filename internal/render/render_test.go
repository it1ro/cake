package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/it1ro/cake/pkg/types"
)

func TestParse(t *testing.T) {
	tests := []struct {
		in      string
		want    Format
		wantErr bool
	}{
		{"xml", FormatXML, false},
		{"XML", FormatXML, false},
		{" markdown ", FormatMarkdown, false},
		{"plain", FormatPlain, false},
		{"yaml", "", true},
		{"", "", true},
	}
	for _, tc := range tests {
		got, err := Parse(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("Parse(%q): want error, got %v", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("Parse(%q): unexpected error: %v", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("Parse(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRender_Dispatch(t *testing.T) {
	ctx := types.Context{
		Project: "p",
		Files: []types.ProcessedFile{
			{Entry: types.FileEntry{Path: "a.go", Language: "go"}, Content: []byte("package a\n")},
		},
	}
	cases := []struct {
		format Format
		want   string
	}{
		{FormatXML, "<context"},
		{FormatMarkdown, "# Context: p"},
		{FormatPlain, "project: p"},
	}
	for _, tc := range cases {
		var buf bytes.Buffer
		if err := Render(ctx, tc.format, &buf); err != nil {
			t.Fatalf("Render(%q): %v", tc.format, err)
		}
		if !strings.Contains(buf.String(), tc.want) {
			t.Errorf("Render(%q): missing %q in output", tc.format, tc.want)
		}
	}
}

// Пустой формат (zero value Options) не должен падать.
func TestRender_EmptyDefaultsToXML(t *testing.T) {
	var buf bytes.Buffer
	if err := Render(types.Context{Project: "p"}, "", &buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "<context") {
		t.Errorf("empty format should default to XML, got: %s", buf.String())
	}
}
