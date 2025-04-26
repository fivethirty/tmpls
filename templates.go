package tmpls

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"sync"
)

type Config struct {
	TemplatesDir string
	TemplatesFS  fs.FS
	CommonGlob   string
}

type executor interface {
	ExecuteTemplate(io.Writer, string, any) error
}

type Templates struct {
	config    Config
	executors sync.Map
	buffers   sync.Pool
	logger    *slog.Logger
}

func New(config Config, logger *slog.Logger) (*Templates, error) {
	if config.TemplatesDir != "" {
		logger.Warn(
			"TemplatesDir is set, templs won't use embedded templates",
			"templatesDir", config.TemplatesDir,
		)
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
	template string,
	data any,
) error {
	value, _ := t.executors.Load(glob)
	if value == nil {
		e, err := t.newExecutor(glob)
		if err != nil {
			return err
		}
		value = e
		t.executors.Store(glob, value)
	}

	executor, ok := value.(executor)
	if !ok {
		return fmt.Errorf("invalid executor type: %T", value)
	}

	return executor.ExecuteTemplate(buffer, template, data)
}

func (t *Templates) newExecutor(patterns ...string) (executor, error) {
	// common goes first so it can be overridden
	allPatterns := append([]string{t.config.CommonGlob}, patterns...)
	if t.config.TemplatesDir != "" {
		return t.newFSExecutor(allPatterns...)
	} else {
		return t.newEmbeddedExecutor(allPatterns...)
	}
}

func (t *Templates) newFSExecutor(patterns ...string) (executor, error) {
	return &fsExecutor{
		templatesDir: t.config.TemplatesDir,
		patterns:     patterns,
	}, nil
}

func (fe *fsExecutor) ExecuteTemplate(wr io.Writer, name string, data any) error {
	t, err := template.ParseFS(
		os.DirFS(fe.templatesDir),
		fe.patterns...,
	)
	if err != nil {
		return err
	}
	return t.ExecuteTemplate(wr, name, data)
}

func (t *Templates) newEmbeddedExecutor(patterns ...string) (executor, error) {
	executor, err := template.ParseFS(t.config.TemplatesFS, patterns...)
	if err != nil {
		return nil, err
	}
	return executor, nil
}

type fsExecutor struct {
	templatesDir string
	patterns     []string
}
