package tmpls_test

import (
	"crypto/rand"
	"embed"
	"encoding/base64"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/fivethirty/tmpls"
)

//go:embed testtemplates
var templatesFS embed.FS

func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		templatesDir string
		templatesFS  fs.FS
		commonGlob   string
		expectError  bool
	}{
		{
			name:         "should create new templates with dir",
			templatesDir: "/foo",
			commonGlob:   "*.html",
			expectError:  false,
		},
		{
			name:         "should create new templates with dir without CommonGlob",
			templatesDir: "/foo",
			expectError:  false,
		},
		{
			name:         "should create new templates with dir and fs set",
			templatesDir: "/foo",
			templatesFS:  templatesFS,
			expectError:  false,
		},
		{
			name:        "should create new templates with fs",
			templatesFS: templatesFS,
			commonGlob:  "*.html",
			expectError: false,
		},
		{
			name:        "should create new templates with fs without CommonGlob",
			templatesFS: templatesFS,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := tmpls.New(
				tmpls.Config{
					TemplatesDir: tt.templatesDir,
					TemplatesFS:  tt.templatesFS,
					CommonGlob:   tt.commonGlob,
				},
				slog.Default(),
			)

			if tt.expectError != (err != nil) {
				t.Fatalf("expectError=%v, got %v", tt.expectError, err)
			}
		})

	}
}

type templateData struct {
	Text string
}

func TestEmbeddedExecute(t *testing.T) {
	t.Parallel()

	subFS, err := fs.Sub(templatesFS, "testtemplates")
	if err != nil {
		t.Fatal(err)
	}

	tmpls, err := tmpls.New(
		tmpls.Config{
			TemplatesFS: subFS,
			CommonGlob:  "common/*.html.tmpl",
		},
		slog.Default(),
	)
	if err != nil {
		t.Fatal(err)
	}

	testExecute(t, tmpls)
}

func TestFSExecute(t *testing.T) {
	t.Parallel()
	dir := copyEmbeddedFS(t)

	tmpls, err := tmpls.New(
		tmpls.Config{
			TemplatesDir: dir,
			CommonGlob:   "common/*.html.tmpl",
		},
		slog.Default(),
	)
	if err != nil {
		t.Fatal(err)
	}

	testExecute(t, tmpls)
}

func testExecute(t *testing.T, tmpls *tmpls.Templates) {
	t.Helper()
	tests := []struct {
		name     string
		glob     string
		template string
		data     templateData
		expected string
	}{
		{
			name:     "should execute template",
			glob:     "test.html.tmpl",
			template: "test.html.tmpl",
			data:     templateData{Text: "world"},
			expected: "hello world",
		},
		{
			name:     "should execute a different template",
			glob:     "other.html.tmpl",
			template: "other.html.tmpl",
			data:     templateData{Text: "universe"},
			expected: "hello universe!!!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			output, err := tmpls.Execute(tt.glob, tt.template, tt.data)
			if err != nil {
				t.Fatal(err)
			}
			if output != tt.expected {
				t.Fatalf("expected %s but got %s", tt.expected, output)
			}
		})
	}
}

func randomString(length int) (string, error) {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func copyEmbeddedFS(t *testing.T) string {
	t.Helper()
	rand, err := randomString(16)
	if err != nil {
		t.Fatal(err)
	}

	dir := fmt.Sprintf("/tmp/%s/", rand)
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		t.Fatal(err)
	}

	err = fs.WalkDir(templatesFS, "testtemplates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			t.Fatal(err)
		}
		targetPath := filepath.Join(dir, path)
		if d.IsDir() {
			err := os.Mkdir(targetPath, os.ModePerm)
			if err != nil {
				return err
			}
		} else {
			sourceFile, err := templatesFS.Open(path)
			if err != nil {
				return err
			}
			defer sourceFile.Close()

			targetFile, err := os.Create(targetPath)
			if err != nil {
				return err
			}
			defer targetFile.Close()

			_, err = io.Copy(targetFile, sourceFile)
			if err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		t.Fatal(err)
	}

	return fmt.Sprintf("%s/testtemplates/", dir)
}

func TestHotSwap(t *testing.T) {
	t.Parallel()
	dir := copyEmbeddedFS(t)

	tmpls, err := tmpls.New(
		tmpls.Config{
			TemplatesDir: dir,
			CommonGlob:   "common/*.html.tmpl",
		},
		slog.Default(),
	)
	if err != nil {
		t.Fatal(err)
	}

	output, err := tmpls.Execute(
		"test.html.tmpl",
		"test.html.tmpl",
		templateData{Text: "world"},
	)
	if err != nil {
		t.Fatal(err)
	}

	if output != "hello world" {
		t.Fatalf("expected hello world but got %s", output)
	}

	newContent := `{{ template "common.html.tmpl" . }}{{ define "content" }}{{ .Text }}?{{ end }}`
	path := filepath.Join(dir, "test.html.tmpl")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(
		path,
		[]byte(newContent),
		os.ModePerm,
	)
	if err != nil {
		t.Fatal(err)
	}

	output, err = tmpls.Execute(
		"test.html.tmpl",
		"test.html.tmpl",
		templateData{Text: "world"},
	)
	if err != nil {
		t.Fatal(err)
	}

	if output != "hello world?" {
		t.Fatalf("expected world but got %s", output)
	}
}

func TestHTMLEscaping(t *testing.T) {
	t.Parallel()
	dir := copyEmbeddedFS(t)

	tmpls, err := tmpls.New(
		tmpls.Config{
			TemplatesDir: dir,
			CommonGlob:   "common/*.html.tmpl",
		},
		slog.Default(),
	)
	if err != nil {
		t.Fatal(err)
	}

	output, err := tmpls.Execute(
		"test.html.tmpl",
		"test.html.tmpl",
		templateData{Text: "<script>alert('hello')</script>"},
	)
	if err != nil {
		t.Fatal(err)
	}

	expected := "hello &lt;script&gt;alert(&#39;hello&#39;)&lt;/script&gt;"

	if output != expected {
		t.Fatalf("expected %s but got %s", expected, output)
	}
}
