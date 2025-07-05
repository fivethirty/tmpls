# tmpls

A thread-safe and lightweight tool for working with go html templates.

## Usage

```go
package main

import (
    "embed"
    "log/slog"
    "os"
    
    "github.com/fivethirty/tmpls"
)

//go:embed templates
var templatesFS embed.FS

func main() {
    // Embedded templates (production)
    tmpls, err := tmpls.New(
        tmpls.Config{
            TemplatesFS: templatesFS,
            CommonGlob:  "common/*.html.tmpl",
        },
        slog.Default(),
    )
    
    // Filesystem templates (development)
    tmpls, err := tmpls.New(
        tmpls.Config{
            TemplatesFS:  os.DirFS("./templates"),
            DisableCache: true, // Enable hot-swapping
            CommonGlob:   "common/*.html.tmpl",
        },
        slog.Default(),
    )
    
    // Execute a template
    output, err := tmpls.Execute(
        "*.html.tmpl",     // glob pattern to parse
        "page.html.tmpl",  // template name to execute
        data,              // template data
    )
}
```

## Config

- `TemplatesFS` - Any `fs.FS` containing templates
- `DisableCache` - Disable caching for hot-swapping (default: false)
- `CommonGlob` - Pattern for common templates included in all parses
