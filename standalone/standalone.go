package standalone

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"mime"
	"net/http"
	"path"
	"text/template"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"

	"github.com/fullstorydev/grpcui"
	"github.com/fullstorydev/grpcui/internal/resources/standalone"
)

// Handler returns an HTTP handler that provides a fully-functional gRPC web
// UI, including the main index (with the HTML form), all needed CSS and JS
// assets, and the handlers that provide schema metadata and perform RPC
// invocations.
//
// All RPC invocations are sent to the given channel. The given target is shown
// in the header of the web UI, to show the user where their requests are being
// sent. The given methods enumerate all supported RPC methods, and the given
// files enumerate all known protobuf (for enumerating all supported message
// types, to support the use of google.protobuf.Any messages).
//
// The returned handler expects to serve resources from "/". If it will instead
// be handling a sub-path (e.g. handling "/rpc-ui/") then use http.StripPrefix.
func Handler(ch grpcdynamic.Channel, target string, methods []*desc.MethodDescriptor, files []*desc.FileDescriptor) http.Handler {
	// TODO(jh): add CSRF protection
	webFormHTML := grpcui.WebFormContents("invoke", "metadata", methods)
	webFormJS := grpcui.WebFormScript()
	webFormCSS := grpcui.WebFormSampleCSS()

	var mux http.ServeMux

	for resourcePath, val := range standalone.GetResources() {
		mux.Handle(resourcePath, &resource{
			Data:        val,
			ContentType: mime.TypeByExtension(path.Ext(resourcePath)),
			ETag:        resourceETags[resourcePath],
		})
	}

	mux.Handle("/grpc-web-form.js", &resource{
		Data: func() []byte {
			return webFormJS
		},
		ContentType: "text/javascript; charset=UTF-8",
		ETag:        computeETag(webFormJS),
	})
	mux.Handle("/grpc-web-form.css", &resource{
		Data: func() []byte {
			return webFormCSS
		},
		ContentType: "text/css; charset=utf-8",
		ETag:        computeETag(webFormCSS),
	})

	indexContents := getIndexContents(target, webFormHTML)
	indexResource := &resource{
		Data: func() []byte {
			return indexContents
		},
		ContentType: "text/html; charset=utf-8",
		ETag:        computeETag(indexContents),
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			indexResource.ServeHTTP(w, r)
		} else {
			http.NotFound(w, r)
		}
	})

	rpcInvokeHandler := http.StripPrefix("/invoke", grpcui.RPCInvokeHandler(ch, methods))
	mux.Handle("/invoke/", rpcInvokeHandler)

	rpcMetadataHandler := grpcui.RPCMetadataHandler(methods, files)
	mux.Handle("/metadata", rpcMetadataHandler)

	return &mux
}

var indexTemplate = template.Must(template.New("index.html").Parse(string(standalone.GetIndexTemplate())))

func getIndexContents(target string, webForm []byte) []byte {
	data := struct {
		Target          string
		WebFormContents string
	}{
		Target:          target,
		WebFormContents: string(webForm),
	}
	var indexBuf bytes.Buffer
	if err := indexTemplate.Execute(&indexBuf, data); err != nil {
		panic(err)
	}
	return indexBuf.Bytes()
}

type resource struct {
	Data        func() []byte
	ContentType string
	ETag        string
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
	w.Write(res.Data())
}

var resourceETags = map[string]string{}

func init() {
	for k, v := range standalone.GetResources() {
		resourceETags[k] = computeETag(v())
	}
}

func computeETag(contents []byte) string {
	hasher := sha256.New()
	hasher.Write(contents)
	return base64.RawURLEncoding.EncodeToString(hasher.Sum(nil))
}
