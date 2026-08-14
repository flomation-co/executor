// Package desktop_common holds the shared SSH transport and on-VM command
// builders for the Desktop actions. It has no Execute function, so the manifest
// generator excludes it from the action registry.
//
// Design: the executor never speaks a GUI protocol (RDP/VNC). It opens a pure-Go
// SSH connection (golang.org/x/crypto/ssh — CGO_ENABLED=0 stays intact) and runs
// commands ON the target VM that do the actual work:
//
//   - Linux:   xdotool (input), scrot (screenshot), ffmpeg -f x11grab (record)
//   - Windows: PowerShell + .NET (input/screenshot), ffmpeg -f gdigrab (record)
//
// Those tools are pre-installed on the VM; whether THEY use C is irrelevant to
// the executor binary. Files (screenshots, recordings) are pulled back as
// base64 over the same SSH exec, so there is no SFTP dependency either.
//
// State that must outlive a single action (a running ffmpeg recorder) lives on
// the VM as a pidfile, not in the executor — so each action opens its own
// short-lived connection and closes it. v1 targets one warm, single-session VM.
package desktop_common

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	"golang.org/x/crypto/ssh"
)

// OS selects the on-VM command flavour.
type OS string

const (
	OSLinux   OS = "linux"
	OSWindows OS = "windows"
)

// Fixed on-VM working paths. v1 is single-session, so fixed names are safe and
// keep record_start/record_stop able to find each other across separate actions.
const (
	linuxShotPath = "/tmp/flo_desktop_shot.png"

	winShotPath = `$env:TEMP\flo_desktop_shot.png`
)

// recLinuxPaths / recWinPaths derive per-recording file paths from a unique id,
// so several recorders can run at once without clobbering each other's pidfile
// or output (the old single fixed path made concurrent recordings overwrite one
// file — corrupting them — and left record_stop unable to target the right one).
// The id is caller-generated and validated to a safe charset, so it is safe to
// interpolate into the path.
func recLinuxPaths(id string) (mp4, pid, logf string) {
	base := "/tmp/flo_desktop_rec_" + id
	return base + ".mp4", base + ".pid", base + ".log"
}

func recWinPaths(id string) (mp4, pid string) {
	base := `$env:TEMP\flo_desktop_rec_` + id
	return base + ".mp4", base + ".pid"
}

// runTimeout bounds a single on-VM command. Record start returns immediately
// (it backgrounds ffmpeg); everything else is fast.
const runTimeout = 120 * time.Second

// Conn is a resolved SSH target plus the OS flavour and X display.
type Conn struct {
	addr    string
	config  *ssh.ClientConfig
	poolKey string // connection identity; see pool.go
	OS      OS
	Display string // X display for Linux (e.g. ":0"); ignored on Windows
}

// ResolveConn builds a Conn from the shared connection inputs (the same schema
// as ssh/run: host, port, username, auth_method, private_key/passphrase or
// password, host_fingerprint) plus os and display.
func ResolveConn(inputs []*core.Connection) (*Conn, error) {
	host := strings.TrimSpace(str("host", inputs))
	if host == "" {
		return nil, fmt.Errorf("host is required")
	}
	user := strings.TrimSpace(str("username", inputs))
	if user == "" {
		return nil, fmt.Errorf("username is required")
	}

	port := int64(22)
	if p := core.FindConnection("port", inputs); p != nil {
		if n := p.Number(); n != nil && *n > 0 {
			port = *n
		}
	}

	auth, err := buildAuth(inputs)
	if err != nil {
		return nil, err
	}

	// Host key verification mirrors ssh/run: a supplied SHA-256 fingerprint is
	// enforced; otherwise the key is accepted but the caller flags the risk.
	hostKey := ssh.InsecureIgnoreHostKey()
	if fp := strings.TrimSpace(str("host_fingerprint", inputs)); fp != "" {
		hostKey = fingerprintCallback(fp)
	}

	osKind := OS(strings.TrimSpace(strings.ToLower(str("os", inputs))))
	if osKind != OSWindows {
		osKind = OSLinux
	}
	display := strings.TrimSpace(str("display", inputs))
	if display == "" {
		display = ":0"
	}

	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))

	// The pool key covers everything that decides *which* connection this is:
	// the target, the identity, the credential, and the host-key policy. The
	// credential is hashed inside poolKeyFor, never stored.
	authMethod := str("auth_method", inputs)
	if authMethod == "" {
		authMethod = "key"
	}
	secret := str("private_key", inputs) + "\x00" + str("passphrase", inputs) + "\x00" + str("password", inputs)

	return &Conn{
		addr: addr,
		config: &ssh.ClientConfig{
			User:            user,
			Auth:            []ssh.AuthMethod{auth},
			HostKeyCallback: hostKey,
			Timeout:         30 * time.Second,
		},
		poolKey: poolKeyFor(addr, user, authMethod, secret, strings.TrimSpace(str("host_fingerprint", inputs))),
		OS:      osKind,
		Display: display,
	}, nil
}

// HostKeyUnverified reports whether the connection skipped host-key
// verification (no fingerprint), so actions can surface the warning.
func HostKeyUnverified(inputs []*core.Connection) bool {
	return strings.TrimSpace(str("host_fingerprint", inputs)) == ""
}

// Run opens a session on a pooled connection, runs command under a deadline,
// and returns stdout/stderr/exit. A non-zero exit is returned via exit (not
// err), like a shell; err is reserved for connection/transport failures.
//
// The connection is reused across actions (see pool.go). A pooled connection
// can die between uses — the VM reboots, sshd restarts, a NAT drops the socket
// — so a transport failure on a *reused* connection is retried exactly once on
// a freshly dialled one. A first-attempt failure on a fresh connection is a
// real failure and is returned as-is.
func (c *Conn) Run(command string) (stdout, stderr string, exit int64, err error) {
	stdout, stderr, exit, err, retryable := c.runOnce(command)
	if err != nil && retryable {
		stdout, stderr, exit, err, _ = c.runOnce(command)
	}
	return stdout, stderr, exit, err
}

// runOnce is a single attempt. retryable reports that the failure was a
// transport error on a connection taken from the pool, so retrying on a fresh
// dial is worthwhile.
func (c *Conn) runOnce(command string) (stdout, stderr string, exit int64, err error, retryable bool) {
	entry := entryFor(c.poolKey)
	client, reused, dialErr := entry.connect(c.addr, c.config)
	if dialErr != nil {
		return "", "", 0, fmt.Errorf("failed to connect to %s: %w", c.addr, dialErr), false
	}

	session, sessErr := client.NewSession()
	if sessErr != nil {
		// Opening a channel failed: the connection is unhealthy (or has hit the
		// server's MaxSessions). Drop it so the retry — or the next action —
		// dials fresh.
		entry.discard(client)
		return "", "", 0, fmt.Errorf("failed to open SSH session: %w", sessErr), reused
	}
	defer func() { _ = session.Close() }()

	var out, errb bytes.Buffer
	session.Stdout = &out
	session.Stderr = &errb

	done := make(chan error, 1)
	go func() { done <- session.Run(command) }()

	timer := time.NewTimer(runTimeout)
	defer timer.Stop()
	select {
	case runErr := <-done:
		if runErr != nil {
			if exitErr, ok := runErr.(*ssh.ExitError); ok {
				// The command ran and exited non-zero: the connection is fine.
				return out.String(), errb.String(), int64(exitErr.ExitStatus()), nil, false
			}
			entry.discard(client)
			return out.String(), errb.String(), 0, fmt.Errorf("run command on %s: %w", c.addr, runErr), reused
		}
		return out.String(), errb.String(), 0, nil, false
	case <-timer.C:
		// A timeout is a real result about the command, not the connection —
		// retrying would just burn another runTimeout. Kill the session and
		// keep the connection for the next action.
		_ = session.Signal(ssh.SIGKILL)
		return "", "", 0, fmt.Errorf("command timed out on %s after %s", c.addr, runTimeout), false
	}
}

// ReadFileBytes fetches a file from the VM as base64 over an SSH exec (no SFTP
// dependency), returning the decoded bytes.
func (c *Conn) ReadFileBytes(remotePath string) ([]byte, error) {
	stdout, stderr, exit, err := c.Run(ReadFileB64Cmd(c.OS, remotePath))
	if err != nil {
		return nil, err
	}
	if exit != 0 {
		return nil, fmt.Errorf("reading %s failed (exit %d): %s", remotePath, exit, strings.TrimSpace(stderr))
	}
	// Strip whitespace/newlines the shell/base64 may introduce.
	clean := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, stdout)
	data, derr := base64.StdEncoding.DecodeString(clean)
	if derr != nil {
		return nil, fmt.Errorf("decoding %s: %w", remotePath, derr)
	}
	return data, nil
}

// --- on-VM command builders (pure functions — unit-tested) ---

// ScreenshotCmd captures the screen to a file on the VM.
func ScreenshotCmd(os OS, display, path string) string {
	if os == OSWindows {
		// .NET screen capture of the primary screen to a PNG. Requires an
		// interactive desktop session (see the plan's Session-0 note).
		return ps(`Add-Type -AssemblyName System.Windows.Forms,System.Drawing;` +
			`$b=[System.Windows.Forms.SystemInformation]::VirtualScreen;` +
			`$bmp=New-Object System.Drawing.Bitmap $b.Width,$b.Height;` +
			`$g=[System.Drawing.Graphics]::FromImage($bmp);` +
			`$g.CopyFromScreen($b.Left,$b.Top,0,0,$bmp.Size);` +
			`$bmp.Save('` + path + `',[System.Drawing.Imaging.ImageFormat]::Png)`)
	}
	// scrot is the lightest X screenshot tool; -o overwrites.
	return fmt.Sprintf("DISPLAY=%s scrot -o %s", shArg(display), shArg(path))
}

// ClickCmd moves the pointer to (x,y) and clicks the given button
// (left/right/middle) clicks times. clicks<=1 is a single click; clicks==2 is a
// double-click. All clicks happen in ONE command (one SSH round-trip) so they
// land within the OS double-click threshold — calling this action twice would
// not, as two SSH exec round-trips are too far apart to register as a double.
func ClickCmd(os OS, display string, x, y int64, button string, clicks int64) string {
	if clicks < 1 {
		clicks = 1
	}
	if os == OSWindows {
		if clicks <= 1 {
			return ps(winMouseType() +
				fmt.Sprintf(`[FloMouse]::Click(%d,%d,'%s')`, x, y, winButton(button)))
		}
		return ps(winMouseType() +
			fmt.Sprintf(`1..%d | ForEach-Object { [FloMouse]::Click(%d,%d,'%s'); Start-Sleep -Milliseconds 40 }`,
				clicks, x, y, winButton(button)))
	}
	if clicks <= 1 {
		return fmt.Sprintf("DISPLAY=%s xdotool mousemove %d %d click %d", shArg(display), x, y, xdoButton(button))
	}
	// xdotool --repeat N issues N clicks in one command; --delay is the ms gap
	// between them (90ms is comfortably inside the default 500ms double-click
	// window while remaining two distinct presses).
	return fmt.Sprintf("DISPLAY=%s xdotool mousemove %d %d click --repeat %d --delay 90 %d",
		shArg(display), x, y, clicks, xdoButton(button))
}

// FocusWindowCmd raises and gives input focus to the first window whose title
// contains `title`. Run before a click/type so the intended window sits on top
// and receives the input, rather than whatever happened to be focused or
// overlapping (e.g. a leftover terminal intercepting clicks). Linux uses
// xdotool windowactivate (needs an EWMH window manager — Xfwm4 etc.); Windows
// uses WScript.Shell.AppActivate (matches on the title). A non-match exits
// non-zero, which callers treat as best-effort.
func FocusWindowCmd(os OS, display, title string) string {
	if os == OSWindows {
		return ps(fmt.Sprintf(`$ok=(New-Object -ComObject WScript.Shell).AppActivate('%s'); if(-not $ok){ exit 1 }`, psLiteral(title)))
	}
	return fmt.Sprintf("DISPLAY=%s xdotool search --limit 1 --name %s windowactivate --sync", shArg(display), shArg(title))
}

// FocusWindowIfRequested reads an optional "window" input and, when set, raises
// and focuses that window (best-effort) so the input action that follows lands
// on it. A non-match is NOT fatal — the caller proceeds regardless, matching the
// pre-existing behaviour when no window is requested.
func (c *Conn) FocusWindowIfRequested(inputs []*core.Connection) {
	window := OptionalString("window", inputs)
	if window == "" {
		return
	}
	_, _, _, _ = c.Run(FocusWindowCmd(c.OS, c.Display, window))
}

// Point is a pointer position. A nil *Point means "wherever the pointer
// already is".
type Point struct{ X, Y int64 }

// ScreenInfoCmd reports the desktop geometry and the active window title, one
// per line: "<width> <height>" then the title.
//
// This exists because the agent's only view of the VM is a screenshot, and a
// screenshot carries no statement of its own scale. Vision models downscale
// images above 1568px on the long edge, so on a large display the coordinates
// an agent reads off a screenshot are silently in a different space from the
// ones ClickCmd feeds to xdotool — every click lands short and the agent has no
// way to see why. Reporting the true geometry makes that checkable in one call.
func ScreenInfoCmd(os OS, display string) string {
	if os == OSWindows {
		return ps(winWindowType() +
			`Add-Type -AssemblyName System.Windows.Forms;` +
			`$b=[System.Windows.Forms.SystemInformation]::VirtualScreen;` +
			`Write-Output ("{0} {1}" -f $b.Width,$b.Height);` +
			`Write-Output ([FloWin]::ActiveTitle())`)
	}
	// `|| true` keeps the exit status clean when no window is active (a bare
	// desktop), which is informational here rather than a failure.
	return fmt.Sprintf("DISPLAY=%s xdotool getdisplaygeometry; DISPLAY=%s xdotool getactivewindow getwindowname 2>/dev/null || true",
		shArg(display), shArg(display))
}

// NavigateCmd drives the already-open browser to a URL in its CURRENT tab, via
// the address bar (Ctrl+L, type, Enter).
//
// OpenURLCmd shells out to the browser binary, which — when the browser is
// already running — hands the URL to the existing instance as a NEW TAB. That
// makes it useless for "go to the next page in this journey": the tab count
// climbs, the previous page stays live behind it, and any attempt to act on
// "the page" becomes ambiguous. This is the same-tab counterpart.
func NavigateCmd(os OS, display, url string) string {
	if os == OSWindows {
		b := base64.StdEncoding.EncodeToString([]byte(url))
		return ps(`Add-Type -AssemblyName System.Windows.Forms;` +
			`[System.Windows.Forms.SendKeys]::SendWait('^l');Start-Sleep -Milliseconds 200;` +
			`$t=[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('` + b + `'));` +
			`$t=[regex]::Replace($t,'[+^%~(){}\[\]]','{$0}');` +
			`[System.Windows.Forms.SendKeys]::SendWait($t);` +
			`[System.Windows.Forms.SendKeys]::SendWait('{ENTER}')`)
	}
	// Focus the address bar, give the browser a moment to select its contents,
	// then type over it. The URL goes through the same base64 path as TypeCmd,
	// so nothing in it can reach the shell.
	return fmt.Sprintf(`DISPLAY=%s xdotool key --clearmodifiers ctrl+l; sleep 0.2; %s`,
		shArg(display), TypeCmd(OSLinux, display, url, true))
}

// MoveCmd moves the pointer without clicking.
func MoveCmd(os OS, display string, x, y int64) string {
	if os == OSWindows {
		return ps(winMouseType() + fmt.Sprintf(`[FloMouse]::Move(%d,%d)`, x, y))
	}
	return fmt.Sprintf("DISPLAY=%s xdotool mousemove %d %d", shArg(display), x, y)
}

// TypeCmd types literal text. The text is base64-encoded and decoded on the VM
// so it is passed as a single argument — no shell/SendKeys metacharacter can be
// injected from the text content. When submit is true it presses Return
// afterwards IN THE SAME command, so "type a command and run it" is one atomic
// action rather than a type + a separate key-press the agent can forget.
func TypeCmd(os OS, display, text string, submit bool) string {
	b := base64.StdEncoding.EncodeToString([]byte(text))
	if os == OSWindows {
		enter := ""
		if submit {
			enter = `;[System.Windows.Forms.SendKeys]::SendWait('{ENTER}')`
		}
		return ps(`Add-Type -AssemblyName System.Windows.Forms;` +
			`$t=[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('` + b + `'));` +
			// Escape SendKeys metacharacters {}()+^%~[] by wrapping each in {}.
			`$t=[regex]::Replace($t,'[+^%~(){}\[\]]','{$0}');` +
			`[System.Windows.Forms.SendKeys]::SendWait($t)` + enter)
	}
	// "$(...)" command substitution is not word-split or re-evaluated, and the
	// base64 alphabet is shell-safe, so the decoded text reaches xdotool as one
	// argument with no injection surface.
	cmd := fmt.Sprintf(`DISPLAY=%s xdotool type --clearmodifiers -- "$(printf %%s '%s' | base64 -d)"`, shArg(display), b)
	if submit {
		cmd += fmt.Sprintf(`; DISPLAY=%s xdotool key --clearmodifiers Return`, shArg(display))
	}
	return cmd
}

// KeyCmd presses a key or chord. Linux uses xdotool key syntax ("ctrl+c",
// "Return"); Windows uses SendKeys syntax ("^c", "{ENTER}").
func KeyCmd(os OS, display, keys string) string {
	if os == OSWindows {
		return ps(`Add-Type -AssemblyName System.Windows.Forms;` +
			`[System.Windows.Forms.SendKeys]::SendWait('` + psLiteral(keys) + `')`)
	}
	return fmt.Sprintf("DISPLAY=%s xdotool key --clearmodifiers %s", shArg(display), shArg(keys))
}

// ScrollCmd scrolls up or down by amount notches, optionally moving the pointer
// to `at` first.
//
// Moving the pointer matters more than it looks. A wheel event is delivered to
// whatever sits UNDER THE POINTER, not to whatever has keyboard focus — so
// scrolling without positioning first scrolls whatever the last click happened
// to leave the pointer over. The common failure is a scrollable dialog (a
// licence agreement, a long modal) that refuses to move because the pointer is
// still parked on the page behind it, which reads as "scrolling is broken"
// rather than "scrolling is going somewhere else".
func ScrollCmd(os OS, display, direction string, amount int64, at *Point) string {
	if amount <= 0 {
		amount = 3
	}
	if os == OSWindows {
		delta := -120 * amount // wheel down is negative
		if strings.EqualFold(direction, "up") {
			delta = 120 * amount
		}
		move := ""
		if at != nil {
			move = fmt.Sprintf(`[FloMouse]::Move(%d,%d);`, at.X, at.Y)
		}
		return ps(winMouseType() + move + fmt.Sprintf(`[FloMouse]::Wheel(%d)`, delta))
	}
	button := 5 // down
	if strings.EqualFold(direction, "up") {
		button = 4
	}
	if at != nil {
		return fmt.Sprintf("DISPLAY=%s xdotool mousemove %d %d click --repeat %d %d",
			shArg(display), at.X, at.Y, amount, button)
	}
	return fmt.Sprintf("DISPLAY=%s xdotool click --repeat %d %d", shArg(display), amount, button)
}

// RunCommandCmd runs an arbitrary shell command on the VM. On Linux it exports
// DISPLAY first (when set) so GUI commands — and every command in an
// &&-chained sequence — target the desktop session; it is harmless for non-GUI
// commands. The command is passed to the VM's shell verbatim (this IS a
// run-arbitrary-command tool, so the caller owns the command).
func RunCommandCmd(os OS, display, command string) string {
	if os == OSWindows {
		return command
	}
	if strings.TrimSpace(display) == "" {
		return command
	}
	return fmt.Sprintf("export DISPLAY=%s; %s", shArg(display), command)
}

// OpenURLCmd opens a URL in a browser on the VM.
//
// The Linux path is deliberately robust: `xdg-open` HANGS when no working
// default handler is configured (a common headless state), which used to time
// the action out. So we (1) launch fully detached — setsid+nohup+& — so a
// blocking browser can never hold the SSH exec open, and (2) prefer a real
// browser binary, falling back to xdg-open only if none is found. Chrome/Chromium
// get --no-sandbox because they refuse to start under the typical
// root/no-user-namespace VM profile otherwise. The URL is passed as a positional
// arg ("$1"), so it is never re-parsed by the shell.
func OpenURLCmd(os OS, display, url string) string {
	if os == OSWindows {
		return ps(`Start-Process '` + psLiteral(url) + `'`)
	}
	const launch = `for b in google-chrome google-chrome-stable chromium chromium-browser; do ` +
		`command -v "$b" >/dev/null 2>&1 && exec "$b" --no-sandbox --no-first-run "$1"; done; ` +
		`command -v firefox >/dev/null 2>&1 && exec firefox "$1"; ` +
		`exec xdg-open "$1"`
	return fmt.Sprintf(`DISPLAY=%s setsid nohup sh -c %s _ %s >/dev/null 2>&1 &`,
		shArg(display), shArg(launch), shArg(url))
}

// RecordStartCmd starts a detached ffmpeg screen recorder for recording `id`,
// writing its PID to a per-id pidfile so RecordStopCmd(id) can find exactly it.
func RecordStartCmd(os OS, display string, fps int64, id string) string {
	if fps <= 0 {
		fps = 15
	}
	if os == OSWindows {
		winRecPath, winRecPID := recWinPaths(id)
		// Start ffmpeg hidden and detached; persist its PID. evenPadFilter keeps
		// the frame dimensions divisible by 2 (libx264 + yuv420p rejects odd
		// dimensions, a common cause of a corrupt/unplayable recording).
		return ps(fmt.Sprintf(
			`$p=Start-Process ffmpeg -WindowStyle Hidden -PassThru -ArgumentList `+
				`'-y','-f','gdigrab','-draw_mouse','1','-framerate','%d','-i','desktop',`+
				`'-vf','%s','-c:v','libx264','-preset','ultrafast','-pix_fmt','yuv420p','-movflags','+faststart','%s';`+
				`$p.Id | Out-File -Encoding ascii '%s'`, fps, evenPadFilter, winRecPath, winRecPID))
	}
	linuxRecPath, linuxRecPID, linuxRecLog := recLinuxPaths(id)
	// setsid+nohup fully detaches ffmpeg so it survives the SSH session close;
	// std streams go to a log so nothing blocks. INT (Ctrl-C) later finalises
	// the moov atom cleanly. -draw_mouse 1 keeps the cursor visible; the even-pad
	// filter guards against odd capture dimensions (libx264/yuv420p corruption);
	// +faststart moves the moov atom to the front for clean streamed playback.
	// The pad filter MUST be shell-quoted: it contains parentheses, which bash
	// treats as syntax (a subshell) and otherwise errors on with
	// "syntax error near unexpected token `('".
	return fmt.Sprintf(
		`DISPLAY=%s setsid nohup ffmpeg -y -f x11grab -draw_mouse 1 -framerate %d -i %s `+
			`-vf %s -c:v libx264 -preset ultrafast -pix_fmt yuv420p -movflags +faststart %s >%s 2>&1 & echo $! > %s`,
		shArg(display), fps, shArg(display), shArg(evenPadFilter), linuxRecPath, linuxRecLog, linuxRecPID)
}

// evenPadFilter pads the captured frame up to the next even width/height. A
// screen grab whose dimensions are odd makes libx264 with yuv420p fail or emit
// a corrupt stream; padding (rather than scaling) keeps every source pixel
// crisp and only ever adds at most one black row/column.
const evenPadFilter = "pad=ceil(iw/2)*2:ceil(ih/2)*2"

// RecordStopCmd signals recording `id`'s recorder to stop and waits for it to
// finalise the file before returning.
func RecordStopCmd(os OS, id string) string {
	if os == OSWindows {
		_, winRecPID := recWinPaths(id)
		return ps(fmt.Sprintf(
			`$pid=Get-Content '%s'; try{ $p=Get-Process -Id $pid -ErrorAction Stop; `+
				`$p.CloseMainWindow() | Out-Null; Start-Sleep -Milliseconds 300; `+
				`if(!$p.HasExited){ Stop-Process -Id $pid -Force }; $p.WaitForExit(10000) }catch{}`, winRecPID))
	}
	_, linuxRecPID, _ := recLinuxPaths(id)
	// SIGINT, then poll until the process is gone (moov atom flushed), capped.
	return fmt.Sprintf(
		`kill -INT "$(cat %s)" 2>/dev/null; `+
			`for i in $(seq 1 20); do kill -0 "$(cat %s)" 2>/dev/null || break; sleep 0.5; done; `+
			`rm -f %s`,
		linuxRecPID, linuxRecPID, linuxRecPID)
}

// ReadFileB64Cmd emits a command that prints a file as single-line base64.
func ReadFileB64Cmd(os OS, path string) string {
	if os == OSWindows {
		return ps(`[Convert]::ToBase64String([IO.File]::ReadAllBytes('` + psLiteral(path) + `'))`)
	}
	return fmt.Sprintf("base64 -w0 %s", shArg(path))
}

// ScreenshotPath / RecordPath expose the fixed on-VM paths to the actions.
func ScreenshotPath(os OS) string {
	if os == OSWindows {
		return winShotPath
	}
	return linuxShotPath
}

func RecordPath(os OS, id string) string {
	if os == OSWindows {
		mp4, _ := recWinPaths(id)
		return mp4
	}
	mp4, _, _ := recLinuxPaths(id)
	return mp4
}

// NewestRecordingIDCmd prints the id of the most-recently-started recorder that
// is still running, or nothing if none. Used as a fallback by Stop Recording
// when no recording_id is supplied (e.g. the id was lost when the agent's
// conversation was compacted) so a recording is always stoppable.
func NewestRecordingIDCmd(os OS) string {
	if os == OSWindows {
		return ps(`Get-ChildItem "$env:TEMP\flo_desktop_rec_*.pid" -ErrorAction SilentlyContinue | ` +
			`Sort-Object LastWriteTime -Descending | ForEach-Object { ` +
			`$rp=Get-Content $_ -ErrorAction SilentlyContinue; ` +
			`if($rp -and (Get-Process -Id $rp -ErrorAction SilentlyContinue)){ ($_.BaseName -replace '^flo_desktop_rec_',''); return } }`)
	}
	// ls -t = newest first; print the first whose PID is still alive.
	return `for f in $(ls -t /tmp/flo_desktop_rec_*.pid 2>/dev/null); do ` +
		`p=$(cat "$f" 2>/dev/null); ` +
		`if kill -0 "$p" 2>/dev/null; then n=$(basename "$f" .pid); echo "${n#flo_desktop_rec_}"; break; fi; done`
}

// ListRecordingIDsCmd prints the id of every still-running recorder, one per
// line (newest first on Linux).
func ListRecordingIDsCmd(os OS) string {
	if os == OSWindows {
		return ps(`Get-ChildItem "$env:TEMP\flo_desktop_rec_*.pid" -ErrorAction SilentlyContinue | ` +
			`Sort-Object LastWriteTime -Descending | ForEach-Object { ` +
			`$rp=Get-Content $_ -ErrorAction SilentlyContinue; ` +
			`if($rp -and (Get-Process -Id $rp -ErrorAction SilentlyContinue)){ ($_.BaseName -replace '^flo_desktop_rec_','') } }`)
	}
	return `for f in $(ls -t /tmp/flo_desktop_rec_*.pid 2>/dev/null); do ` +
		`p=$(cat "$f" 2>/dev/null); ` +
		`if kill -0 "$p" 2>/dev/null; then n=$(basename "$f" .pid); echo "${n#flo_desktop_rec_}"; fi; done`
}

// NewRecordingID returns a short, filename/shell-safe unique id for a recording.
func NewRecordingID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// rand.Read essentially never fails; fall back to a time-free constant
		// suffix so the caller still gets a usable (if less unique) id.
		return "rec"
	}
	return hex.EncodeToString(b[:])
}

// SafeRecordingID keeps only [A-Za-z0-9_-]; an empty/invalid id yields "".
// Recording ids are generated by NewRecordingID, but record_stop receives the
// id as an input, so validate defensively before it reaches a shell path.
func SafeRecordingID(id string) string {
	id = strings.TrimSpace(id)
	for _, r := range id {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-') {
			return ""
		}
	}
	return id
}

// --- result shapers ---

// ErrResult builds a failed AI-native result (tool_result mirrors error).
func ErrResult(msg string) map[string]interface{} {
	return map[string]interface{}{"tool_result": msg, "success": false, "error": msg}
}

// OkResult builds a successful result from a summary plus extra outputs.
func OkResult(summary string, extra map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{"tool_result": summary, "success": true, "error": ""}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

// --- input helpers ---

func str(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	return *c.String()
}

// OptionalString reads a trimmed string input ("" when absent).
func OptionalString(name string, inputs []*core.Connection) string {
	return strings.TrimSpace(str(name, inputs))
}

// Int reads an integer input, returning def when absent.
func Int(name string, inputs []*core.Connection, def int64) int64 {
	c := core.FindConnection(name, inputs)
	if c == nil {
		return def
	}
	if n := c.Number(); n != nil {
		return *n
	}
	return def
}

// OptionalBool reads a boolean input, defaulting to false when absent. It also
// accepts the string "true" (after ${...} substitution a bool input can arrive
// as a string), so a wired variable resolves correctly.
func OptionalBool(name string, inputs []*core.Connection) bool {
	c := core.FindConnection(name, inputs)
	if c == nil {
		return false
	}
	if b := c.Boolean(); b != nil {
		return *b
	}
	if s := c.String(); s != nil {
		return strings.EqualFold(strings.TrimSpace(*s), "true")
	}
	return false
}

// buildAuth mirrors ssh/run: key (with optional passphrase) or password.
func buildAuth(inputs []*core.Connection) (ssh.AuthMethod, error) {
	method := str("auth_method", inputs)
	if method == "" {
		method = "key"
	}
	switch method {
	case "password":
		password := str("password", inputs)
		if password == "" {
			return nil, fmt.Errorf("password is required for password authentication")
		}
		return ssh.Password(password), nil
	case "key":
		key := str("private_key", inputs)
		if key == "" {
			return nil, fmt.Errorf("private key is required for key authentication")
		}
		var signer ssh.Signer
		var err error
		if passphrase := str("passphrase", inputs); passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(key), []byte(passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey([]byte(key))
		}
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		return ssh.PublicKeys(signer), nil
	default:
		return nil, fmt.Errorf("unknown authentication method: %s", method)
	}
}

func fingerprintCallback(expected string) ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		if got := ssh.FingerprintSHA256(key); got != expected {
			return fmt.Errorf("host key mismatch for %s: got %s, expected %s", hostname, got, expected)
		}
		return nil
	}
}

// --- quoting/escaping helpers ---

// shArg single-quotes an argument for POSIX sh, escaping embedded single quotes.
// Used only for values we control (display, fixed paths, xdotool keyspecs); free
// text goes through the base64 path instead.
func shArg(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// ps wraps a PowerShell script for invocation via OpenSSH's default shell.
func ps(script string) string {
	return `powershell -NoProfile -NonInteractive -Command "` + strings.ReplaceAll(script, `"`, `\"`) + `"`
}

// psLiteral escapes a value for a PowerShell single-quoted string (double the
// single quotes).
func psLiteral(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func xdoButton(button string) int {
	switch strings.ToLower(button) {
	case "right":
		return 3
	case "middle":
		return 2
	default:
		return 1
	}
}

func winButton(button string) string {
	switch strings.ToLower(button) {
	case "right", "middle", "left":
		return strings.ToLower(button)
	default:
		return "left"
	}
}

// maxSettle caps the post-action settle. A settle is a convenience, not a
// scheduling primitive — capping it means a mistyped input (30000 meaning
// "30 seconds" but read as milliseconds, or the reverse) cannot stall a flow.
const maxSettle = 10 * time.Second

// Settle pauses after an interaction so the UI can finish reacting before the
// next action — most usefully before the screenshot that the agent will judge
// the result by.
//
// Without it, the screenshot after a click routinely catches the page mid
// animation: a menu part-way open, a page mid scroll, a spinner still up. The
// agent then reasons about a transient frame as though it were the settled
// result, concludes the click did nothing, and retries — which is how one
// mis-timed screenshot turns into a loop of them.
func Settle(ms int64) {
	if ms <= 0 {
		return
	}
	d := time.Duration(ms) * time.Millisecond
	if d > maxSettle {
		d = maxSettle
	}
	time.Sleep(d)
}

// SettleAfter reads the optional settle_ms input and applies it.
func SettleAfter(inputs []*core.Connection) {
	Settle(Int("settle_ms", inputs, 0))
}

// PointFrom reads optional x/y inputs, returning nil when neither is set so the
// caller can distinguish "scroll at (0,0)" from "scroll where the pointer is".
func PointFrom(inputs []*core.Connection, xName, yName string) *Point {
	xc := core.FindConnection(xName, inputs)
	yc := core.FindConnection(yName, inputs)
	if xc == nil && yc == nil {
		return nil
	}
	// A field left blank in the editor arrives as a present-but-empty
	// connection, which must read as "unset" rather than as zero.
	xs, ys := OptionalString(xName, inputs), OptionalString(yName, inputs)
	if xs == "" && ys == "" {
		return nil
	}
	return &Point{X: Int(xName, inputs, 0), Y: Int(yName, inputs, 0)}
}

// winWindowType is a PowerShell Add-Type block defining [FloWin::ActiveTitle].
func winWindowType() string {
	return `Add-Type @'
using System;using System.Text;using System.Runtime.InteropServices;
public class FloWin{
 [DllImport("user32.dll")] static extern IntPtr GetForegroundWindow();
 [DllImport("user32.dll",CharSet=CharSet.Unicode)] static extern int GetWindowText(IntPtr h,StringBuilder s,int n);
 public static string ActiveTitle(){var sb=new StringBuilder(512);GetWindowText(GetForegroundWindow(),sb,512);return sb.ToString();}
}
'@;`
}

// winMouseType is a PowerShell Add-Type block defining [FloMouse] with Move,
// Click and Wheel via user32.dll P/Invoke. Prepended to mouse commands.
func winMouseType() string {
	return `Add-Type @'
using System;using System.Runtime.InteropServices;
public class FloMouse{
 [DllImport("user32.dll")] static extern bool SetCursorPos(int x,int y);
 [DllImport("user32.dll")] static extern void mouse_event(uint f,uint x,uint y,uint d,int e);
 const uint MOVE=0x0001,LD=0x0002,LU=0x0004,RD=0x0008,RU=0x0010,MD=0x0020,MU=0x0040,WHEEL=0x0800;
 public static void Move(int x,int y){SetCursorPos(x,y);}
 public static void Click(int x,int y,string b){SetCursorPos(x,y);
  uint d=LD,u=LU; if(b=="right"){d=RD;u=RU;} else if(b=="middle"){d=MD;u=MU;}
  mouse_event(d,0,0,0,0);mouse_event(u,0,0,0,0);}
 public static void Wheel(int delta){mouse_event(WHEEL,0,0,(uint)delta,0);}
}
'@;`
}
