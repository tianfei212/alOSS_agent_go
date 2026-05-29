package oss

import "testing"

func TestBuildImageProcess(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
		format string
		want   string
	}{
		{"empty", 0, 0, "", ""},
		{"webp only", 0, 0, "webp", "image/format,webp"},
		{"webp uppercase", 0, 0, "WEBP", "image/format,webp"},
		{"resize only", 200, 100, "", "image/resize,w_200,h_100,m_fill"},
		{"resize and webp", 200, 100, "webp", "image/resize,w_200,h_100,m_fill/format,webp"},
		{"partial width ignored", 200, 0, "webp", "image/format,webp"},
		{"partial height ignored", 0, 100, "webp", "image/format,webp"},
		{"unknown format ignored", 0, 0, "png", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildImageProcess(tt.width, tt.height, tt.format)
			if got != tt.want {
				t.Fatalf("BuildImageProcess(%d, %d, %q) = %q, want %q", tt.width, tt.height, tt.format, got, tt.want)
			}
		})
	}
}

func TestIsImageKey(t *testing.T) {
	if !IsImageKey("photos/a.jpg") {
		t.Fatal("expected jpg to be image")
	}
	if IsImageKey("videos/a.mp4") {
		t.Fatal("expected mp4 not to be image")
	}
	if !IsImageKey("x.WEBP") {
		t.Fatal("expected webp extension to be image")
	}
}
