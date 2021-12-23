package services

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"path"
	"path/filepath"
	"runtime"
	"sync"

	"goweb/config"
	"goweb/funcmap"
)

// TemplateRenderer provides a flexible and easy to use method of rendering simple templates or complex sets of
// templates while also providing caching and/or hot-reloading depending on your current environment
type TemplateRenderer struct {
	// templateCache stores a cache of parsed page templates
	templateCache sync.Map

	// funcMap stores the template function map
	funcMap template.FuncMap

	// templatePath stores the complete path to the templates directory
	templatesPath string

	// config stores application configuration
	config *config.Config
}

// NewTemplateRenderer creates a new TemplateRenderer
func NewTemplateRenderer(cfg *config.Config) *TemplateRenderer {
	t := &TemplateRenderer{
		templateCache: sync.Map{},
		funcMap:       funcmap.GetFuncMap(),
		config:        cfg,
	}

	// Gets the complete templates directory path
	// This is needed incase this is called from a package outside of main, such as within tests
	_, b, _, _ := runtime.Caller(0)
	d := path.Join(path.Dir(b))
	t.templatesPath = filepath.Join(filepath.Dir(d), config.TemplateDir)

	return t
}

func (t *TemplateRenderer) ParseAndExecute(group, id, name string, files []string, directories []string, data interface{}) (*bytes.Buffer, error) {
	var buf *bytes.Buffer
	var err error

	if err = t.Parse(group, id, name, files, directories); err != nil {
		return nil, err
	}
	if buf, err = t.Execute(group, id, name, data); err != nil {
		return nil, err
	}

	return buf, nil
}

func (t *TemplateRenderer) Parse(group, id, name string, files []string, directories []string) error {
	cacheKey := t.getCacheKey(group, id)

	// Check if the template has not yet been parsed or if the app environment is local, so that
	// templates reflect changes without having the restart the server
	if _, err := t.Load(group, id); err != nil || t.config.App.Environment == config.EnvLocal {
		// Initialize the parsed template with the function map
		parsed := template.New(name + config.TemplateExt).
			Funcs(t.funcMap)

		// Parse all files provided
		if len(files) > 0 {
			for k, v := range files {
				files[k] = fmt.Sprintf("%s/%s%s", t.templatesPath, v, config.TemplateExt)
			}

			parsed, err = parsed.ParseFiles(files...)
			if err != nil {
				return err
			}
		}

		// Parse all templates within the provided directories
		for _, dir := range directories {
			dir = fmt.Sprintf("%s/%s/*%s", t.templatesPath, dir, config.TemplateExt)
			parsed, err = parsed.ParseGlob(dir)
			if err != nil {
				return err
			}
		}

		// Store the template so this process only happens once
		t.templateCache.Store(cacheKey, parsed)
	}

	return nil
}

func (t *TemplateRenderer) Execute(group, id, name string, data interface{}) (*bytes.Buffer, error) {
	tmpl, err := t.Load(group, id)
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	err = tmpl.ExecuteTemplate(buf, name+config.TemplateExt, data)
	if err != nil {
		return nil, err
	}

	return buf, nil
}

func (t *TemplateRenderer) Load(group, id string) (*template.Template, error) {
	load, ok := t.templateCache.Load(t.getCacheKey(group, id))
	if !ok {
		return nil, errors.New("uncached page template requested")
	}

	tmpl, ok := load.(*template.Template)
	if !ok {
		return nil, errors.New("unable to cast cached template")
	}

	return tmpl, nil
}

func (t *TemplateRenderer) GetTemplatesPath() string {
	return t.templatesPath
}

func (t *TemplateRenderer) getCacheKey(group, id string) string {
	return fmt.Sprintf("%s:%s", group, id)
}
