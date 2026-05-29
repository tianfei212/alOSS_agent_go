package oss

import (
	"strconv"
	"strings"
)

var imageExtensions = map[string]struct{}{
	".jpg":  {},
	".jpeg": {},
	".png":  {},
	".gif":  {},
	".webp": {},
	".bmp":  {},
	".tiff": {},
	".tif":  {},
	".heic": {},
	".avif": {},
}

// IsImageKey reports whether objectKey looks like a raster image OSS can process.
func IsImageKey(objectKey string) bool {
	ext := objectKeyExtension(objectKey)
	_, ok := imageExtensions[ext]
	return ok
}

func objectKeyExtension(objectKey string) string {
	lower := strings.ToLower(objectKey)
	for i := len(lower) - 1; i >= 0; i-- {
		if lower[i] == '.' {
			return lower[i:]
		}
		if lower[i] == '/' {
			break
		}
	}
	return ""
}

// BuildImageProcess builds the OSS x-oss-process string for resize and/or format conversion.
// width and height must both be > 0 to enable resize. format "webp" adds format conversion.
func BuildImageProcess(width, height int, format string) string {
	var parts []string
	if width > 0 && height > 0 {
		parts = append(parts, "resize,w_"+strconv.Itoa(width)+",h_"+strconv.Itoa(height)+",m_fill")
	}
	if strings.EqualFold(format, "webp") {
		parts = append(parts, "format,webp")
	}
	if len(parts) == 0 {
		return ""
	}
	return "image/" + strings.Join(parts, "/")
}
