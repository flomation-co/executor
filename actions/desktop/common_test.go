package desktop_common

import (
	"encoding/base64"
	"os/exec"
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
	Expect(ClickCmd(OSLinux, ":0", 100, 200, "left", 1)).To(Equal("DISPLAY=':0' xdotool mousemove 100 200 click 1"))
	Expect(ClickCmd(OSLinux, ":0", 5, 6, "right", 1)).To(ContainSubstring("click 3"))
	Expect(ClickCmd(OSLinux, ":0", 5, 6, "middle", 1)).To(ContainSubstring("click 2"))
	// clicks < 1 is treated as a single click.
	Expect(ClickCmd(OSLinux, ":0", 1, 2, "left", 0)).To(Equal("DISPLAY=':0' xdotool mousemove 1 2 click 1"))
	// Double-click: both presses in one command, within the OS threshold.
	dbl := ClickCmd(OSLinux, ":0", 7, 8, "left", 2)
	Expect(dbl).To(ContainSubstring("mousemove 7 8 click --repeat 2 --delay 90 1"))
	Expect(MoveCmd(OSLinux, ":0", 10, 20)).To(Equal("DISPLAY=':0' xdotool mousemove 10 20"))

	win := ClickCmd(OSWindows, "", 10, 20, "right", 1)
	Expect(win).To(ContainSubstring("[FloMouse]::Click(10,20,'right')"))
	winDbl := ClickCmd(OSWindows, "", 10, 20, "left", 2)
	Expect(winDbl).To(ContainSubstring("1..2 | ForEach-Object"))
	Expect(winDbl).To(ContainSubstring("[FloMouse]::Click(10,20,'left')"))
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
	// The filter contains parentheses, so it MUST be shell-quoted or bash errors
	// with "syntax error near unexpected token `('".
	Expect(start).To(ContainSubstring("-vf " + shArg(evenPadFilter)))
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

// TestLinuxCommandsAreValidShell parses every generated Linux command with
// `bash -n` (syntax check, no execution). This is the regression guard for
// shell-quoting bugs — e.g. an unquoted ffmpeg `-vf pad=ceil(iw/2)*2:...`
// filter whose parentheses bash reads as a subshell and rejects.
func TestLinuxCommandsAreValidShell(t *testing.T) {
	RegisterTestingT(t)
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not available")
	}
	cmds := map[string]string{
		"screenshot":   ScreenshotCmd(OSLinux, ":0", "/tmp/s.png"),
		"click":        ClickCmd(OSLinux, ":0", 10, 20, "left", 1),
		"double_click": ClickCmd(OSLinux, ":0", 10, 20, "left", 2),
		"move":         MoveCmd(OSLinux, ":0", 10, 20),
		"type":         TypeCmd(OSLinux, ":0", `weird "'()$ text`),
		"key":          KeyCmd(OSLinux, ":0", "ctrl+shift+t"),
		"scroll":       ScrollCmd(OSLinux, ":0", "down", 3),
		"open_url":     OpenURLCmd(OSLinux, ":0", "https://example.com/a?b=c&d=(e)"),
		"record_start": RecordStartCmd(OSLinux, ":0", 15),
		"record_stop":  RecordStopCmd(OSLinux),
	}
	for name, c := range cmds {
		out, err := exec.Command(bash, "-n", "-c", c).CombinedOutput()
		Expect(err).To(BeNil(), "%s is not valid shell:\n  cmd: %s\n  err: %s", name, c, string(out))
	}
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
