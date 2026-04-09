package antivirus

import (
	"os/exec"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func strConn(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

func boolConn(name string, value bool) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeBoolean, Value: value}
}

func skipIfNoClamAV(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("clamscan"); err != nil {
		t.Skip("clamscan not installed, skipping test")
	}
}

func Test_ScanEmptyDirectory(t *testing.T) {
	skipIfNoClamAV(t)
	RegisterTestingT(t)

	dir := t.TempDir()

	result, err := Execute(nil, nil, []*core.Connection{
		strConn("scan_path", dir),
		boolConn("recursive", false),
	})
	Expect(err).To(BeNil())
	Expect(result["is_clean"]).To(Equal(true))
	Expect(result["infected_count"]).To(Equal(0))
}

// Test_ScanEICARTestFile removed — the EICAR test string triggers local
// endpoint protection (e.g. macOS XProtect, Windows Defender) which
// quarantines the file on disk write before ClamAV can scan it.

func Test_ScanNonExistentPath(t *testing.T) {
	RegisterTestingT(t)

	_, err := Execute(nil, nil, []*core.Connection{
		strConn("scan_path", "/nonexistent/path/that/does/not/exist"),
	})
	Expect(err).ToNot(BeNil())
}
