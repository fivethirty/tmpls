package tmpls_test

import (
	"io/fs"
	"log/slog"
	"testing"
	"testing/fstest"

	"github.com/fivethirty/tmpls"
)

var testFS = fstest.MapFS{
	"test.html.tmpl": &fstest.MapFile{
		Data: []byte(
			`{{ template "common.html.tmpl" . }}{{ define "content" }}{{ .Text }}{{ end }}`,
		),
	},
	"other.html.tmpl": &fstest.MapFile{
		Data: []byte(
			`{{ template "common.html.tmpl" . }}{{ define "content" }}{{ .Text }}!!!{{ end }}`,
		),
	},
	"common/common.html.tmpl": &fstest.MapFile{
		Data: []byte(`hello {{ template "content" . }}`),
	},
}

func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		templatesFS  fs.FS
		disableCache bool
		commonGlob   string
		expectError  bool
	}{
		{
			name:        "should create new templates",
			templatesFS: testFS,
			commonGlob:  "*.html.tmpl",
			expectError: false,
		},
		{
			name:        "should create new templates without CommonGlob",
			templatesFS: testFS,
			expectError: false,
		},
		{
			name:         "should create new templates with cache disabled",
			templatesFS:  testFS,
			disableCache: true,
			commonGlob:   "*.html.tmpl",
			expectError:  false,
		},
		{
			name:        "should fail with nil FS",
			templatesFS: nil,
			expectError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := tmpls.New(
				tmpls.Config{
					TemplatesFS:  test.templatesFS,
					DisableCache: test.disableCache,
					CommonGlob:   test.commonGlob,
				},
				slog.Default(),
			)

			if test.expectError != (err != nil) {
				t.Fatalf("expectError=%v, got %v", test.expectError, err)
			}
		})
	}
}

type templateData struct {
	Text string
}

func TestCachedExecute(t *testing.T) {
	t.Parallel()

	tmpls, err := tmpls.New(
		tmpls.Config{
			TemplatesFS: testFS,
			CommonGlob:  "common/*.html.tmpl",
		},
		slog.Default(),
	)
	if err != nil {
		t.Fatal(err)
	}

	testExecute(t, tmpls)
}

func TestNonCachedExecute(t *testing.T) {
	t.Parallel()

	tmpls, err := tmpls.New(
		tmpls.Config{
			TemplatesFS:  testFS,
			DisableCache: true,
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

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			output, err := tmpls.Execute(test.glob, test.template, test.data)
			if err != nil {
				t.Fatal(err)
			}
			if output != test.expected {
				t.Fatalf("expected %s but got %s", test.expected, output)
			}
		})
	}
}

func TestHotSwap(t *testing.T) {
	t.Parallel()

	mutableFS := fstest.MapFS{
		"test.html.tmpl": &fstest.MapFile{
			Data: []byte(
				`{{ template "common.html.tmpl" . }}{{ define "content" }}{{ .Text }}{{ end }}`,
			),
		},
		"common/common.html.tmpl": &fstest.MapFile{
			Data: []byte(`hello {{ template "content" . }}`),
		},
	}

	tmpls, err := tmpls.New(
		tmpls.Config{
			TemplatesFS:  mutableFS,
			DisableCache: true,
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

	mutableFS["test.html.tmpl"] = &fstest.MapFile{
		Data: []byte(
			`{{ template "common.html.tmpl" . }}{{ define "content" }}{{ .Text }}?{{ end }}`,
		),
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
		t.Fatalf("expected hello world? but got %s", output)
	}
}

func TestHTMLEscaping(t *testing.T) {
	t.Parallel()

	tmpls, err := tmpls.New(
		tmpls.Config{
			TemplatesFS: testFS,
			CommonGlob:  "common/*.html.tmpl",
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
