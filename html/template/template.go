package template

import (
	"html/template"
	"io"

	"github.com/labstack/echo/v4"
)

type Renderer struct {
	*template.Template
}

func (r *Renderer) Render(w io.Writer, name string, data interface{}, _ echo.Context) error {
	return r.ExecuteTemplate(w, name, data)
}
