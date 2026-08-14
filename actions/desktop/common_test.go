package desktop_common

import (
	"encoding/base64"
	"os/exec"
	"strings"
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
	cmd := TypeCmd(OSLinux, ":0", evil, false)
	Expect(cmd).ToNot(ContainSubstring("rm -rf"))
	Expect(cmd).To(ContainSubstring(base64.StdEncoding.EncodeToString([]byte(evil))))
	Expect(cmd).To(ContainSubstring("base64 -d"))
	Expect(cmd).To(ContainSubstring("xdotool type --clearmodifiers"))
	// Without submit, no Return keypress.
	Expect(cmd).ToNot(ContainSubstring("key --clearmodifiers Return"))

	// submit=true appends a Return in the same command.
	withEnter := TypeCmd(OSLinux, ":0", "ls -la", true)
	Expect(withEnter).To(ContainSubstring("xdotool type --clearmodifiers"))
	Expect(withEnter).To(ContainSubstring("xdotool key --clearmodifiers Return"))

	win := TypeCmd(OSWindows, "", "hi", false)
	Expect(win).To(ContainSubstring(base64.StdEncoding.EncodeToString([]byte("hi"))))
	Expect(win).To(ContainSubstring("SendKeys"))
	Expect(TypeCmd(OSWindows, "", "hi", true)).To(ContainSubstring("{ENTER}"))
}

func TestKeyCmd(t *testing.T) {
	RegisterTestingT(t)
	Expect(KeyCmd(OSLinux, ":0", "ctrl+c")).To(Equal("DISPLAY=':0' xdotool key --clearmodifiers 'ctrl+c'"))
	Expect(KeyCmd(OSWindows, "", "^c")).To(ContainSubstring("SendKeys"))
}

func TestFocusWindowCmd(t *testing.T) {
	RegisterTestingT(t)
	Expect(FocusWindowCmd(OSLinux, ":0", "Google Chrome")).
		To(Equal("DISPLAY=':0' xdotool search --limit 1 --name 'Google Chrome' windowactivate --sync"))
	// Title is shell-quoted, so spaces/metacharacters (incl. parens) are safe.
	Expect(FocusWindowCmd(OSLinux, ":0", "a & (b)")).To(ContainSubstring("--name 'a & (b)'"))
	win := FocusWindowCmd(OSWindows, "", "Chrome")
	Expect(win).To(ContainSubstring("AppActivate('Chrome')"))
}

func TestScrollCmd(t *testing.T) {
	RegisterTestingT(t)
	Expect(ScrollCmd(OSLinux, ":0", "down", 3, nil)).To(Equal("DISPLAY=':0' xdotool click --repeat 3 5"))
	Expect(ScrollCmd(OSLinux, ":0", "up", 2, nil)).To(ContainSubstring("--repeat 2 4"))
	Expect(ScrollCmd(OSLinux, ":0", "down", 0, nil)).To(ContainSubstring("--repeat 3 5")) // default amount
	Expect(ScrollCmd(OSWindows, "", "up", 1, nil)).To(ContainSubstring("Wheel(120)"))
	Expect(ScrollCmd(OSWindows, "", "down", 1, nil)).To(ContainSubstring("Wheel(-120)"))
}

// A wheel event goes to whatever is under the POINTER, so a scroll that does
// not position first silently scrolls whatever the last click left it over —
// the reason a scrollable modal appears frozen while the page behind it moves.
func TestScrollCmd_PositionsPointerFirst(t *testing.T) {
	RegisterTestingT(t)

	linux := ScrollCmd(OSLinux, ":0", "down", 4, &Point{X: 640, Y: 400})
	Expect(linux).To(Equal("DISPLAY=':0' xdotool mousemove 640 400 click --repeat 4 5"))
	// The move must precede the wheel clicks, not follow them.
	Expect(strings.Index(linux, "mousemove")).To(BeNumerically("<", strings.Index(linux, "click")))

	// Anchor on the INVOCATIONS ("[FloMouse]::…"), not the method names — the
	// prepended Add-Type block declares both Move and Wheel, so a bare name
	// matches the declaration and says nothing about call order.
	win := ScrollCmd(OSWindows, "", "up", 1, &Point{X: 12, Y: 34})
	Expect(win).To(ContainSubstring("::Move(12,34)"))
	Expect(strings.Index(win, "::Move(12,34)")).To(BeNumerically("<", strings.Index(win, "::Wheel(")))
}

func TestScreenInfoCmd(t *testing.T) {
	RegisterTestingT(t)
	linux := ScreenInfoCmd(OSLinux, ":0")
	Expect(linux).To(ContainSubstring("getdisplaygeometry"))
	Expect(linux).To(ContainSubstring("getactivewindow getwindowname"))
	// A bare desktop with nothing focused is informational, not a failure.
	Expect(linux).To(ContainSubstring("|| true"))
	Expect(ScreenInfoCmd(OSWindows, "")).To(ContainSubstring("VirtualScreen"))
	Expect(ScreenInfoCmd(OSWindows, "")).To(ContainSubstring("ActiveTitle"))
}

// NavigateCmd drives the address bar so a journey stays in one tab, rather than
// handing the URL to the running browser as a new one.
func TestNavigateCmd(t *testing.T) {
	RegisterTestingT(t)

	linux := NavigateCmd(OSLinux, ":0", "https://example.com/a?b=c&d=(e)")
	Expect(linux).To(HavePrefix("DISPLAY=':0' xdotool key --clearmodifiers ctrl+l"))
	Expect(linux).To(ContainSubstring("base64 -d")) // URL never reaches the shell verbatim
	Expect(linux).NotTo(ContainSubstring("example.com"))
	Expect(linux).To(ContainSubstring("key --clearmodifiers Return")) // submits

	win := NavigateCmd(OSWindows, "", "https://example.com")
	Expect(win).To(ContainSubstring("SendWait('^l')"))
	Expect(win).To(ContainSubstring("{ENTER}"))
	Expect(win).To(ContainSubstring("FromBase64String"))
}

func TestOpenURLCmd(t *testing.T) {
	RegisterTestingT(t)
	linux := OpenURLCmd(OSLinux, ":0", "https://x/y")
	// Detached so it can never hang the action; tries real browsers then xdg-open.
	Expect(linux).To(HavePrefix("DISPLAY=':0' setsid nohup sh -c"))
	Expect(linux).To(HaveSuffix("&"))
	Expect(linux).To(ContainSubstring("google-chrome"))
	Expect(linux).To(ContainSubstring("--no-sandbox"))
	Expect(linux).To(ContainSubstring("exec xdg-open"))
	Expect(linux).To(ContainSubstring("'https://x/y'")) // URL passed as a quoted positional arg
	Expect(OpenURLCmd(OSWindows, "", "https://x/y")).To(ContainSubstring("Start-Process 'https://x/y'"))
}

func TestRunCommandCmd(t *testing.T) {
	RegisterTestingT(t)
	// Linux with a display: DISPLAY exported so GUI + &&-chained commands target
	// the session; the command itself is passed verbatim.
	Expect(RunCommandCmd(OSLinux, ":0", "wget -qO /tmp/b.png http://x && echo done")).
		To(Equal("export DISPLAY=':0'; wget -qO /tmp/b.png http://x && echo done"))
	// No display: run verbatim.
	Expect(RunCommandCmd(OSLinux, "", "ls -la")).To(Equal("ls -la"))
	// Windows: verbatim.
	Expect(RunCommandCmd(OSWindows, "", "Get-Process")).To(Equal("Get-Process"))
}

func TestRecordCmds(t *testing.T) {
	RegisterTestingT(t)
	const id = "abc123"
	_, pid, _ := recLinuxPaths(id)

	start := RecordStartCmd(OSLinux, ":0", 20, id)
	Expect(start).To(ContainSubstring("ffmpeg -y -f x11grab -draw_mouse 1 -framerate 20"))
	Expect(start).To(ContainSubstring("setsid nohup"))
	Expect(start).To(ContainSubstring(pid))
	// The recording id is in the path, so concurrent recorders don't clobber.
	Expect(start).To(ContainSubstring("flo_desktop_rec_" + id))
	Expect(start).To(ContainSubstring("-vf " + shArg(evenPadFilter)))
	Expect(start).To(ContainSubstring("+faststart"))
	Expect(RecordStartCmd(OSLinux, ":0", 0, id)).To(ContainSubstring("-framerate 15")) // default fps
	// Distinct ids → distinct commands (isolated files).
	Expect(RecordStartCmd(OSLinux, ":0", 15, "id1")).ToNot(Equal(RecordStartCmd(OSLinux, ":0", 15, "id2")))

	stop := RecordStopCmd(OSLinux, id)
	Expect(stop).To(ContainSubstring("kill -INT"))
	Expect(stop).To(ContainSubstring(pid))

	win := RecordStartCmd(OSWindows, "", 30, id)
	Expect(win).To(ContainSubstring("gdigrab"))
	Expect(win).To(ContainSubstring(evenPadFilter))
	Expect(win).To(ContainSubstring("+faststart"))
	Expect(RecordStopCmd(OSWindows, id)).To(ContainSubstring("Stop-Process"))
}

func TestRecordingID(t *testing.T) {
	RegisterTestingT(t)
	Expect(NewRecordingID()).ToNot(Equal(NewRecordingID())) // unique
	Expect(NewRecordingID()).To(MatchRegexp("^[0-9a-f]+$"))
	Expect(SafeRecordingID("abc-123_XY")).To(Equal("abc-123_XY"))
	Expect(SafeRecordingID("  ok  ")).To(Equal("ok"))
	Expect(SafeRecordingID("bad; rm -rf /")).To(Equal("")) // shell metachars rejected
	Expect(SafeRecordingID("")).To(Equal(""))
}

func TestRecordingResolveCmds(t *testing.T) {
	RegisterTestingT(t)
	// Both resolve/list commands scan the per-id pidfiles and only report a
	// recorder whose PID is still alive.
	newest := NewestRecordingIDCmd(OSLinux)
	Expect(newest).To(ContainSubstring("/tmp/flo_desktop_rec_*.pid"))
	Expect(newest).To(ContainSubstring("kill -0"))
	Expect(newest).To(ContainSubstring("flo_desktop_rec_")) // strips the prefix to get the id
	Expect(newest).To(ContainSubstring("break"))            // stops at the first (newest) alive one

	list := ListRecordingIDsCmd(OSLinux)
	Expect(list).To(ContainSubstring("/tmp/flo_desktop_rec_*.pid"))
	Expect(list).To(ContainSubstring("kill -0"))
	Expect(list).ToNot(ContainSubstring("break")) // lists ALL, not just the newest

	Expect(NewestRecordingIDCmd(OSWindows)).To(ContainSubstring("Get-ChildItem"))
	Expect(ListRecordingIDsCmd(OSWindows)).To(ContainSubstring("Get-ChildItem"))
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
		"focus_window": FocusWindowCmd(OSLinux, ":0", "a & (b) window"),
		"run_command":  RunCommandCmd(OSLinux, ":0", "wget -qO /tmp/b.png 'http://x/(y)' && echo done"),
		"newest_rec":   NewestRecordingIDCmd(OSLinux),
		"list_rec":     ListRecordingIDsCmd(OSLinux),
		"click":        ClickCmd(OSLinux, ":0", 10, 20, "left", 1),
		"double_click": ClickCmd(OSLinux, ":0", 10, 20, "left", 2),
		"move":         MoveCmd(OSLinux, ":0", 10, 20),
		"type":         TypeCmd(OSLinux, ":0", `weird "'()$ text`, false),
		"type_submit":  TypeCmd(OSLinux, ":0", `echo hi && ls`, true),
		"key":          KeyCmd(OSLinux, ":0", "ctrl+shift+t"),
		"scroll":       ScrollCmd(OSLinux, ":0", "down", 3, nil),
		"scroll_at":    ScrollCmd(OSLinux, ":0", "down", 3, &Point{X: 10, Y: 20}),
		"screen_info":  ScreenInfoCmd(OSLinux, ":0"),
		"navigate":     NavigateCmd(OSLinux, ":0", "https://example.com/a?b=c&d=(e)"),
		"open_url":     OpenURLCmd(OSLinux, ":0", "https://example.com/a?b=c&d=(e)"),
		"record_start": RecordStartCmd(OSLinux, ":0", 15, "t1"),
		"record_stop":  RecordStopCmd(OSLinux, "t1"),
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
