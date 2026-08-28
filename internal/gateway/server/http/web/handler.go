package web

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
)


type GatewayError struct {
	StatusCode int
	Title      string
	Message    string
}

var (
	ErrBadRequest = GatewayError{
		StatusCode: http.StatusBadRequest,
		Title:      "Bad Request",
		Message:     "The request could not be processed.",
	}

	ErrUnauthorized = GatewayError{
		StatusCode: http.StatusUnauthorized,
		Title:      "Authentication Required",
		Message:     "You must authenticate before accessing this application.",
	}

	ErrForbidden = GatewayError{
		StatusCode: http.StatusForbidden,
		Title:      "Access Denied",
		Message:     "You are not authorized to access this application.",
	}

	ErrNotFound = GatewayError{
		StatusCode: http.StatusNotFound,
		Title:      "Application Not Found",
		Message:     "The application you requested could not be found.",
	}

	ErrBadGateway = GatewayError{
		StatusCode: http.StatusBadGateway,
		Title:      "Application Unavailable",
		Message:     "The gateway could not reach the application.",
	}

	ErrServiceUnavailable = GatewayError{
		StatusCode: http.StatusServiceUnavailable,
		Title:      "Gateway Unavailable",
		Message:     "The gateway is temporarily unable to process your request.",
	}

	ErrInternal = GatewayError{
		StatusCode: http.StatusInternalServerError,
		Title:      "Gateway Error",
		Message:     "The gateway encountered an unexpected error.",
	}
)

//go:embed *
var idpTemplateFS embed.FS

type ErrorHandler struct {
	templates  *template.Template
}

func NewErrorHandler() *ErrorHandler {

	tmpl := template.Must(
		template.New("error").ParseFS(
			idpTemplateFS,
			"templates/error.html",
		),
	)

	return &ErrorHandler{
		templates:  tmpl,
	}
}

// ErrorPage renders an HTML error page.
func (h *ErrorHandler) ErrorPage(w http.ResponseWriter, statusCode int, title, message, requestID string) {

	data := struct {
		StatusCode  int
		Title       string
		Message     string
		RequestID   string
	}{
		StatusCode:  statusCode,
		Title:       title,
		Message:     message,
		RequestID:   requestID,
	}

	w.WriteHeader(statusCode)
	if err := h.templates.ExecuteTemplate(w, "error.html", data); err != nil {
		http.Error(w, "failed to render error page", http.StatusInternalServerError)
	}
}

//Static File Server


func NewStaticFileHandler() http.Handler {
	staticFS, err := fs.Sub(idpTemplateFS, "static")
	if err != nil {
		panic(err)
	}
	
	return http.StripPrefix("/_ashrix/static/",http.FileServer(http.FS(staticFS)))
}	