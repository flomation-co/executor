package list_targets

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
)

func TestParseMakefile_BasicTargets(t *testing.T) {
	RegisterTestingT(t)

	content := `
.PHONY: build test clean

# Build the project
build: main.go utils.go
	go build -o app .

# Run unit tests
test: build
	go test ./...

clean:
	rm -f app
`
	path := writeTempMakefile(t, content)
	targets, err := parseMakefile(path)
	Expect(err).ToNot(HaveOccurred())
	Expect(targets).To(HaveLen(3))

	Expect(targets[0].Name).To(Equal("build"))
	Expect(targets[0].Description).To(Equal("Build the project"))
	Expect(targets[0].Dependencies).To(Equal([]string{"main.go", "utils.go"}))
	Expect(targets[0].Phony).To(BeTrue())

	Expect(targets[1].Name).To(Equal("test"))
	Expect(targets[1].Description).To(Equal("Run unit tests"))
	Expect(targets[1].Dependencies).To(Equal([]string{"build"}))
	Expect(targets[1].Phony).To(BeTrue())

	Expect(targets[2].Name).To(Equal("clean"))
	Expect(targets[2].Description).To(BeEmpty())
	Expect(targets[2].Phony).To(BeTrue())
}

func TestParseMakefile_SkipsVariables(t *testing.T) {
	RegisterTestingT(t)

	content := `
CC = gcc
CFLAGS := -Wall -O2
LDFLAGS ?= -lm
VERSION += 1.0

build:
	$(CC) $(CFLAGS) -o app main.c
`
	path := writeTempMakefile(t, content)
	targets, err := parseMakefile(path)
	Expect(err).ToNot(HaveOccurred())
	Expect(targets).To(HaveLen(1))
	Expect(targets[0].Name).To(Equal("build"))
}

func TestParseMakefile_SkipsPatternRules(t *testing.T) {
	RegisterTestingT(t)

	content := `
%.o: %.c
	gcc -c $< -o $@

build: main.o
	gcc -o app main.o
`
	path := writeTempMakefile(t, content)
	targets, err := parseMakefile(path)
	Expect(err).ToNot(HaveOccurred())
	Expect(targets).To(HaveLen(1))
	Expect(targets[0].Name).To(Equal("build"))
}

func TestParseMakefile_NoDependencies(t *testing.T) {
	RegisterTestingT(t)

	content := `
all:
	echo "hello"
`
	path := writeTempMakefile(t, content)
	targets, err := parseMakefile(path)
	Expect(err).ToNot(HaveOccurred())
	Expect(targets).To(HaveLen(1))
	Expect(targets[0].Name).To(Equal("all"))
	Expect(targets[0].Dependencies).To(BeNil())
}

func TestParseMakefile_EmptyFile(t *testing.T) {
	RegisterTestingT(t)

	path := writeTempMakefile(t, "# Just a comment\n")
	targets, err := parseMakefile(path)
	Expect(err).ToNot(HaveOccurred())
	Expect(targets).To(BeEmpty())
}

func TestParseMakefile_PhonyAfterTarget(t *testing.T) {
	RegisterTestingT(t)

	content := `
build:
	go build .

.PHONY: build
`
	path := writeTempMakefile(t, content)
	targets, err := parseMakefile(path)
	Expect(err).ToNot(HaveOccurred())
	Expect(targets).To(HaveLen(1))
	Expect(targets[0].Name).To(Equal("build"))
	Expect(targets[0].Phony).To(BeTrue())
}

func TestFindMakefile(t *testing.T) {
	RegisterTestingT(t)

	dir := t.TempDir()

	// No Makefile exists
	Expect(findMakefile(dir)).To(BeEmpty())

	// Create Makefile
	_ = os.WriteFile(filepath.Join(dir, "Makefile"), []byte("all:\n"), 0644)
	Expect(findMakefile(dir)).To(Equal(filepath.Join(dir, "Makefile")))
}

func writeTempMakefile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "Makefile")
	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}
	return path
}
