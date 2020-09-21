package standalone

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"html/template"
	"mime"
	"net/http"
	"path"
	"strings"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	"google.golang.org/grpc"

	"github.com/fullstorydev/grpcui"
	"github.com/fullstorydev/grpcui/internal/resources/standalone"
)

const csrfCookieName = "_grpcui_csrf_token"
const csrfHeaderName = "x-grpcui-csrf-token"

// Handler returns an HTTP handler that provides a fully-functional gRPC web
// UI, including the main index (with the HTML form), all needed CSS and JS
// assets, and the handlers that provide schema metadata and perform RPC
// invocations. The HTML index, CSS, and JS files can be customized and
// augmented with opts.
//
// All RPC invocations are sent to the given channel. The given target is shown
// in the header of the web UI, to show the user where their requests are being
// sent. The given methods enumerate all supported RPC methods, and the given
// files enumerate all known protobuf (for enumerating all supported message
// types, to support the use of google.protobuf.Any messages).
//
// The returned handler expects to serve resources from "/". If it will instead
// be handling a sub-path (e.g. handling "/rpc-ui/") then use http.StripPrefix.
func Handler(ch grpcdynamic.Channel, target string, methods []*desc.MethodDescriptor, files []*desc.FileDescriptor, opts ...HandlerOption) http.Handler {
	uiOpts := &handlerOptions{
		indexTmpl: defaultIndexTemplate,
		css:       grpcui.WebFormSampleCSS(),
	}
	for _, o := range opts {
		o.apply(uiOpts)
	}

	var mux http.ServeMux

	// Add go-bindata resources bundled in standalone package TOC
	for _, assetName := range standalone.AssetNames() {
		// the index file will be handled separately
		if assetName == standalone.IndexTemplateName {
			continue
		}
		resourcePath := "/" + assetName
		handle(&mux, newResource(resourcePath, standalone.MustAsset(assetName), ""))
	}

	// Add resources from WebFormPackage
	handle(&mux, newResource("/grpc-web-form.js", grpcui.WebFormScript(), "text/javascript; charset=UTF-8"))
	handle(&mux, newResource("/grpc-web-form.css", uiOpts.css, "text/css; charset=UTF-8"))

	// Add optional resources to mux
	for _, res := range uiOpts.addlServedResources() {
		handle(&mux, res)
	}

	// Add the index page (not bundled in standalone)
	formOpts := grpcui.WebFormOptions{
		DefaultMetadata: uiOpts.defaultMetadata,
		Debug:           uiOpts.debug,
	}
	webFormHTML := grpcui.WebFormContentsWithOptions("invoke", "metadata", methods, formOpts)
	indexContents := getIndexContents(uiOpts.indexTmpl, target, webFormHTML, uiOpts.tmplResources)
	indexResource := newResource("/", indexContents, "text/html; charset=utf-8")
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			indexResource.ServeHTTP(w, r)
		} else {
			http.NotFound(w, r)
		}
	})

	rpcInvokeHandler := http.StripPrefix("/invoke", grpcui.RPCInvokeHandler(ch, methods))
	mux.HandleFunc("/invoke/", func(w http.ResponseWriter, r *http.Request) {
		// CSRF protection
		c, _ := r.Cookie(csrfCookieName)
		h := r.Header.Get(csrfHeaderName)
		if c == nil || c.Value == "" || c.Value != h {
			http.Error(w, "incorrect CSRF token", http.StatusUnauthorized)
			return
		}
		rpcInvokeHandler.ServeHTTP(w, r)
	})

	rpcMetadataHandler := grpcui.RPCMetadataHandler(methods, files)
	mux.Handle("/metadata", rpcMetadataHandler)

	// make sure we always have a csrf token cookie
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := r.Cookie(csrfCookieName); err != nil {
			tokenBytes := make([]byte, 32)
			if _, err := rand.Read(tokenBytes); err != nil {
				http.Error(w, "failed to create CSRF token", http.StatusInternalServerError)
				return
			}
			c := &http.Cookie{
				Name:  csrfCookieName,
				Value: base64.RawURLEncoding.EncodeToString(tokenBytes),
			}
			http.SetCookie(w, c)
		}

		mux.ServeHTTP(w, r)
	})
}

var defaultIndexTemplate = template.Must(template.New("index.html").Parse(string(standalone.IndexTemplate())))

func getIndexContents(tmpl *template.Template, target string, webFormHTML []byte, addlResources []*resource) []byte {
	addlHTML := make([]template.HTML, 0, len(addlResources))
	for _, res := range addlResources {
		tag := res.AsHTMLTag()
		if tag != "" {
			addlHTML = append(addlHTML, template.HTML(tag))
		}
	}
	data := WebFormContainerTemplateData{
		Target:          target,
		WebFormContents: template.HTML(webFormHTML),
		AddlResources:   addlHTML,
	}
	var indexBuf bytes.Buffer
	if err := tmpl.Execute(&indexBuf, data); err != nil {
		panic(err)
	}
	return indexBuf.Bytes()
}

type resource struct {
	Path        string
	Data        []byte
	ContentType string
	ETag        string
}

func newResource(uriPath string, data []byte, contentType string) *resource {
	if contentType == "" {
		contentType = mime.TypeByExtension(path.Ext(uriPath))
	}
	return &resource{
		Path:        uriPath,
		Data:        data,
		ContentType: contentType,
		ETag:        computeETag(data),
	}
}

func handle(mux *http.ServeMux, res *resource) {
	mux.Handle(res.Path, res)
}

func (res *resource) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	etag := r.Header.Get("If-None-Match")
	if etag != "" && etag == res.ETag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", res.ContentType)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("ETag", res.ETag)
	_, _ = w.Write(res.Data)
}

// AsHTMLTag returns an HTML string corresponding to a tag that would load this resource (by inspecting ContentType).
// Only supports "text/javascript" and "text/css" for ContentType.
// Returns empty string if we do not support the ContentType.
func (res *resource) AsHTMLTag() string {
	if strings.HasPrefix(res.ContentType, "text/javascript") {
		return fmt.Sprintf("<script src=\"%s\"></script>", strings.TrimLeft(res.Path, "/"))
	} else if strings.HasPrefix(res.ContentType, "text/css") {
		return fmt.Sprintf("<link rel=\"stylesheet\" href=\"%s\">", strings.TrimLeft(res.Path, "/"))
	}

	// Fallthrough as a no-op
	return ""
}

func computeETag(contents []byte) string {
	hasher := sha256.New()
	hasher.Write(contents)
	return base64.RawURLEncoding.EncodeToString(hasher.Sum(nil))
}

// HandlerViaReflection tries to query the provided connection for all services
// and methods supported by the server, and constructs a handler to serve the UI.
//
// The handler has the same properties as the one returned by Handler.
func HandlerViaReflection(ctx context.Context, cc grpc.ClientConnInterface, target string, opts ...HandlerOption) (http.Handler, error) {
	m, err := grpcui.AllMethodsViaReflection(ctx, cc)
	if err != nil {
		return nil, err
	}

	f, err := grpcui.AllFilesViaReflection(ctx, cc)
	if err != nil {
		return nil, err
	}

	return Handler(cc, target, m, f, opts...), nil
}
