package desktop_common

import (
	"encoding/base64"
	"testing"

	. "github.com/onsi/gomega"
)

func TestScreenshotCmd(t *testing.T) {
	RegisterTestingT(t)
	Expect(ScreenshotCmd(OSLinux, ":0", "/tmp/s.png")).To(Equal("DISPLAY=':0' scrot -o '/tmp/s.png'"))
	win := ScreenshotCmd(OSWindows, "", winShotPath)
	Expect(win).To(HavePrefix("powershell -NoProfile -NonInteractive -Command"))
	Expect(win).To(ContainSubstring("CopyFromScreen"))
}

func TestClickAndMoveCmd(t *testing.T) {
	RegisterTestingT(t)
	Expect(ClickCmd(OSLinux, ":0", 100, 200, "left")).To(Equal("DISPLAY=':0' xdotool mousemove 100 200 click 1"))
	Expect(ClickCmd(OSLinux, ":0", 5, 6, "right")).To(ContainSubstring("click 3"))
	Expect(ClickCmd(OSLinux, ":0", 5, 6, "middle")).To(ContainSubstring("click 2"))
	Expect(MoveCmd(OSLinux, ":0", 10, 20)).To(Equal("DISPLAY=':0' xdotool mousemove 10 20"))

	win := ClickCmd(OSWindows, "", 10, 20, "right")
	Expect(win).To(ContainSubstring("[FloMouse]::Click(10,20,'right')"))
}

func TestTypeCmd_IsInjectionSafe(t *testing.T) {
	RegisterTestingT(t)
	// A hostile string must NOT appear literally in the command — it is passed
	// base64-encoded and decoded on the VM into a single argument.
	evil := `"; rm -rf / #`
	cmd := TypeCmd(OSLinux, ":0", evil)
	Expect(cmd).ToNot(ContainSubstring("rm -rf"))
	Expect(cmd).To(ContainSubstring(base64.StdEncoding.EncodeToString([]byte(evil))))
	Expect(cmd).To(ContainSubstring("base64 -d"))
	Expect(cmd).To(ContainSubstring("xdotool type --clearmodifiers"))

	win := TypeCmd(OSWindows, "", "hi")
	Expect(win).To(ContainSubstring(base64.StdEncoding.EncodeToString([]byte("hi"))))
	Expect(win).To(ContainSubstring("SendKeys"))
}

func TestKeyCmd(t *testing.T) {
	RegisterTestingT(t)
	Expect(KeyCmd(OSLinux, ":0", "ctrl+c")).To(Equal("DISPLAY=':0' xdotool key --clearmodifiers 'ctrl+c'"))
	Expect(KeyCmd(OSWindows, "", "^c")).To(ContainSubstring("SendKeys"))
}

func TestScrollCmd(t *testing.T) {
	RegisterTestingT(t)
	Expect(ScrollCmd(OSLinux, ":0", "down", 3)).To(Equal("DISPLAY=':0' xdotool click --repeat 3 5"))
	Expect(ScrollCmd(OSLinux, ":0", "up", 2)).To(ContainSubstring("--repeat 2 4"))
	Expect(ScrollCmd(OSLinux, ":0", "down", 0)).To(ContainSubstring("--repeat 3 5")) // default amount
	Expect(ScrollCmd(OSWindows, "", "up", 1)).To(ContainSubstring("Wheel(120)"))
	Expect(ScrollCmd(OSWindows, "", "down", 1)).To(ContainSubstring("Wheel(-120)"))
}

func TestOpenURLCmd(t *testing.T) {
	RegisterTestingT(t)
	Expect(OpenURLCmd(OSLinux, ":0", "https://x/y")).To(Equal("DISPLAY=':0' xdg-open 'https://x/y'"))
	Expect(OpenURLCmd(OSWindows, "", "https://x/y")).To(ContainSubstring("Start-Process 'https://x/y'"))
}

func TestRecordCmds(t *testing.T) {
	RegisterTestingT(t)
	start := RecordStartCmd(OSLinux, ":0", 20)
	Expect(start).To(ContainSubstring("ffmpeg -y -f x11grab -draw_mouse 1 -framerate 20"))
	Expect(start).To(ContainSubstring("setsid nohup"))
	Expect(start).To(ContainSubstring(linuxRecPID))
	// Even-dimension pad guards against libx264/yuv420p corruption on odd sizes.
	Expect(start).To(ContainSubstring("-vf " + evenPadFilter))
	Expect(start).To(ContainSubstring("+faststart"))
	Expect(RecordStartCmd(OSLinux, ":0", 0)).To(ContainSubstring("-framerate 15")) // default fps

	stop := RecordStopCmd(OSLinux)
	Expect(stop).To(ContainSubstring("kill -INT"))
	Expect(stop).To(ContainSubstring(linuxRecPID))

	win := RecordStartCmd(OSWindows, "", 30)
	Expect(win).To(ContainSubstring("gdigrab"))
	Expect(win).To(ContainSubstring(evenPadFilter))
	Expect(win).To(ContainSubstring("+faststart"))
	Expect(RecordStopCmd(OSWindows)).To(ContainSubstring("Stop-Process"))
}

func TestReadFileB64Cmd(t *testing.T) {
	RegisterTestingT(t)
	Expect(ReadFileB64Cmd(OSLinux, "/tmp/f")).To(Equal("base64 -w0 '/tmp/f'"))
	Expect(ReadFileB64Cmd(OSWindows, `C:\f`)).To(ContainSubstring("ToBase64String"))
}

func TestShArgEscaping(t *testing.T) {
	RegisterTestingT(t)
	Expect(shArg("simple")).To(Equal("'simple'"))
	// embedded single quote is safely closed-escaped-reopened
	Expect(shArg("a'b")).To(Equal(`'a'\''b'`))
}
