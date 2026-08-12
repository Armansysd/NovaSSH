package desktop

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"time"
)

// OpenWindow launches a native borderless app window using MS Edge / Chrome / Chromium App Mode,
// or falls back to the default OS window opener.
func OpenWindow(url string, width int, height int, enabled bool) {
	if !enabled {
		return
	}

	// Wait 500ms for HTTP server to bind
	go func() {
		time.Sleep(500 * time.Millisecond)
		err := openNativeApp(url, width, height)
		if err != nil {
			log.Printf("[Desktop] Native app window fallback: %v", err)
			_ = openFallback(url)
		}
	}()
}

func openNativeApp(url string, width, height int) error {
	windowSize := fmt.Sprintf("--window-size=%d,%d", width, height)
	appUrl := fmt.Sprintf("--app=%s", url)

	switch runtime.GOOS {
	case "windows":
		candidates := []string{
			// Microsoft Edge (Installed by default on all Windows 10 & 11 as WebView2 engine)
			`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
			`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
			// Google Chrome
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
			// Brave Browser
			`C:\Program Files\BraveSoftware\Brave-Browser\Application\brave.exe`,
		}
		for _, exe := range candidates {
			if _, err := os.Stat(exe); err == nil {
				cmd := exec.Command(exe, appUrl, windowSize)
				return cmd.Start()
			}
		}
		return fmt.Errorf("no supported chromium/edge executable found")

	case "linux":
		candidates := []string{
			"google-chrome",
			"google-chrome-stable",
			"chromium",
			"chromium-browser",
			"microsoft-edge",
			"brave-browser",
		}
		for _, bin := range candidates {
			if path, err := exec.LookPath(bin); err == nil {
				cmd := exec.Command(path, appUrl, windowSize)
				return cmd.Start()
			}
		}
		return fmt.Errorf("no supported browser executable found in PATH")

	case "darwin":
		candidates := []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
		}
		for _, exe := range candidates {
			if _, err := os.Stat(exe); err == nil {
				cmd := exec.Command(exe, appUrl, windowSize)
				return cmd.Start()
			}
		}
		return fmt.Errorf("no supported macos browser app found")
	}

	return fmt.Errorf("unsupported operating system for app mode")
}

func openFallback(url string) error {
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("cmd", "/c", "start", url)
		return cmd.Start()
	case "linux":
		cmd := exec.Command("xdg-open", url)
		return cmd.Start()
	case "darwin":
		cmd := exec.Command("open", url)
		return cmd.Start()
	}
	return fmt.Errorf("no fallback opener")
}
