package antivirus

import (
	"os"
	"os/exec"
	"path/filepath"
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

func Test_ScanEICARTestFile(t *testing.T) {
	skipIfNoClamAV(t)
	RegisterTestingT(t)

	dir := t.TempDir()
	// EICAR test string — standard antivirus test signature
	eicar := `X5O!P%@AP[4\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*`
	err := os.WriteFile(filepath.Join(dir, "eicar.txt"), []byte(eicar), 0644)
	Expect(err).To(BeNil())

	result, err := Execute(nil, nil, []*core.Connection{
		strConn("scan_path", dir),
		boolConn("recursive", true),
	})
	Expect(err).To(BeNil())
	Expect(result["is_clean"]).To(Equal(false))
	Expect(result["infected_count"]).To(Equal(1))
}

func Test_ScanNonExistentPath(t *testing.T) {
	RegisterTestingT(t)

	_, err := Execute(nil, nil, []*core.Connection{
		strConn("scan_path", "/nonexistent/path/that/does/not/exist"),
	})
	Expect(err).ToNot(BeNil())
}
