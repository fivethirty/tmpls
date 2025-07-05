package tmpls

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"sync"
)

type Config struct {
	TemplatesFS  fs.FS
	DisableCache bool
	CommonGlob   string
}


type Templates struct {
	config    Config
	executors sync.Map
	buffers   sync.Pool
	logger    *slog.Logger
}

func New(config Config, logger *slog.Logger) (*Templates, error) {
	if config.TemplatesFS == nil {
		return nil, fmt.Errorf("TemplatesFS is required")
	}
	if config.DisableCache {
		logger.Warn("Template caching disabled - templates will be parsed on each request")
	}
	return &Templates{
		config:    config,
		executors: sync.Map{},
		buffers: sync.Pool{
			New: func() any {
				return &bytes.Buffer{}
			},
		},
		logger: logger,
	}, nil
}

func (t *Templates) Execute(
	glob string,
	template string,
	data any,
) (string, error) {
	buffer := t.buffers.Get().(*bytes.Buffer)
	defer func() {
		buffer.Reset()
		t.buffers.Put(buffer)
	}()
	if err := t.execute(buffer, glob, template, data); err != nil {
		return "", err
	}
	return buffer.String(), nil
}

func (t *Templates) execute(
	buffer *bytes.Buffer,
	glob string,
	templateName string,
	data any,
) error {
	if t.config.DisableCache {
		e, err := t.newExecutor(glob)
		if err != nil {
			return err
		}
		return e.ExecuteTemplate(buffer, templateName, data)
	} else {
		value, _ := t.executors.Load(glob)
		var tmpl *template.Template
		if value == nil {
			var err error
			tmpl, err = t.newExecutor(glob)
			if err != nil {
				return err
			}
			t.executors.Store(glob, tmpl)
		} else {
			tmpl = value.(*template.Template)
		}

		return tmpl.ExecuteTemplate(buffer, templateName, data)
	}
}

func (t *Templates) newExecutor(patterns ...string) (*template.Template, error) {
	// common goes first so it can be overridden
	allPatterns := append([]string{t.config.CommonGlob}, patterns...)
	return template.ParseFS(t.config.TemplatesFS, allPatterns...)
}
