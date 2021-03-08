package standalone

import (
	"html/template"
	"path"
)

// WebFormContainerTemplateData is the param type for templates that embed the webform HTML.
// If you use WithIndexTemplate to provide an alternate HTML template for Handler, the template
// should expect a value of this type.
type WebFormContainerTemplateData struct {
	// Target is the name of the machine we are making requests to (for display purposes).
	Target string

	// WebFormContents is the generated form HTML from your ServiceDescriptors.
	WebFormContents template.HTML

	// AddlResources are additional CSS and JS files, in the form of <link> and <script>
	// tags, that we want to append to the HEAD of the index template.
	AddlResources []template.HTML
}

// HandlerOption instances allow for configuration of the standalone Handler.
type HandlerOption interface {
	apply(opts *handlerOptions)
}

// WithIndexTemplate replace the default HTML template used in Handler with the one
// given. The template will be provided an instance of WebFormContainerTemplateData
// as the data to render.
func WithIndexTemplate(tmpl *template.Template) HandlerOption {
	return optFunc(func(opts *handlerOptions) {
		opts.indexTmpl = tmpl
	})
}

// WithCSS entirely replaces the WebFormCSS bytes used by default in Handler.
func WithCSS(css []byte) HandlerOption {
	return optFunc(func(opts *handlerOptions) {
		opts.css = css
	})
}

// AddJS adds a JS file to Handler, serving the supplied contents at the URI
// "/s/<filename>" with a Content-Type of "text/javascript; charset=UTF-8". It
// will also be added to the AddlResources field of the WebFormContainerTemplateData
// so that it is rendered into the HEAD of the HTML page.
//
// It is safe to pass in multiple AddJS Opts to Handler. Each will be rendered in
// the order they are passed.
func AddJS(filename string, js []byte) HandlerOption {
	return optFunc(func(opts *handlerOptions) {
		opts.tmplResources = append(opts.tmplResources, newResource(path.Join("/s", filename), js, "text/javascript; charset=utf-8"))
	})
}

// AddCSS adds a CSS file to Handler, serving the supplied contents at the URI
// "/s/<filename>" with a Content-Type of "text/css; charset=UTF-8". It
// will also be added to the AddlResources field of the WebFormContainerTemplateData
// so that it is rendered into the HEAD of the HTML page.
//
// It is safe to pass in multiple AddCSS Opts to Handler. Each will be rendered in
// the order they are passed.
func AddCSS(filename string, css []byte) HandlerOption {
	return optFunc(func(opts *handlerOptions) {
		opts.tmplResources = append(opts.tmplResources, newResource(path.Join("/s", filename), css, "text/css; charset=utf-8"))
	})
}

// ServeAsset will add an additional file to Hadler, serving the supplied contents
// at the URI "/s/<filename>" with a Content-Type that is computed based on the given
// filename's extension.
//
// These assets could be images or other files referenced by a custom index template.
// Unlike files added via AddJS or AddCSS, they will NOT be provided to the template
// in the AddlResources field of the WebFormContainerTemplateData.
func ServeAsset(filename string, contents []byte) HandlerOption {
	return optFunc(func(opts *handlerOptions) {
		opts.servedOnlyResources = append(opts.servedOnlyResources, newResource(path.Join("/s", filename), contents, ""))
	})
}

// WithDefaultMetadata sets the default metadata in the web form to the given
// values. Each string should be in the form "name: value".
func WithDefaultMetadata(headers []string) HandlerOption {
	return optFunc(func(opts *handlerOptions) {
		opts.defaultMetadata = headers
	})
}

// WithDebug enables console logging in the client JS. This prints extra
// information as the UI processes user input.
func WithDebug(debug bool) HandlerOption {
	return optFunc(func(opts *handlerOptions) {
		opts.debug = &debug
	})
}

// optFunc implements HandlerOption
type optFunc func(opts *handlerOptions)

func (f optFunc) apply(opts *handlerOptions) {
	f(opts)
}

type handlerOptions struct {
	indexTmpl           *template.Template
	css                 []byte
	tmplResources       []*resource
	servedOnlyResources []*resource
	defaultMetadata     []string
	debug               *bool
}

func (opts *handlerOptions) addlServedResources() []*resource {
	return append(opts.tmplResources, opts.servedOnlyResources...)
}
