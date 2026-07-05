package images

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// clipboardScript extracts PNG image data from the macOS clipboard and writes it
// to the file path passed as the first argument. It returns "OK" on success or
// "NO_IMAGE" when the clipboard holds no image.
const clipboardScript = `on run argv
set theFile to item 1 of argv
try
	set imgData to (the clipboard as «class PNGf»)
on error
	return "NO_IMAGE"
end try
set fileRef to (open for access (POSIX file theFile) with write permission)
try
	write imgData to fileRef
end try
close access fileRef
return "OK"
end run`

// PasteClipboardImage writes an image from the system clipboard to destPath and
// returns destPath. It returns an error if the clipboard holds no image or the
// platform is unsupported. Implemented for macOS via osascript; other platforms
// should paste an image file path instead.
func PasteClipboardImage(destPath string) (string, error) {
	if runtime.GOOS != "darwin" {
		return "", fmt.Errorf("clipboard image paste is only supported on macOS; type or paste an image file path instead")
	}
	out, err := exec.Command("osascript", "-e", clipboardScript, destPath).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("reading clipboard: %s", strings.TrimSpace(string(out)))
	}
	if strings.TrimSpace(string(out)) != "OK" {
		return "", fmt.Errorf("no image found in clipboard (copy an image, then Ctrl+V)")
	}
	return destPath, nil
}
