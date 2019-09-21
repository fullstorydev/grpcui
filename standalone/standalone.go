package standalone

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"mime"
	"net/http"
	"path"
	"text/template"

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
	webFormHTML := grpcui.WebFormContents("invoke", "metadata", methods)
	webFormJS := grpcui.WebFormScript()
	webFormCSS := grpcui.WebFormSampleCSS()

	var mux http.ServeMux

	for _, assetName := range standalone.AssetNames() {
		// the index file will be handled separately
		if assetName == "index-template.html" {
			continue
		}
		resourcePath := "/" + assetName
		mux.Handle(resourcePath, &resource{
			Data:        standalone.MustAsset(assetName),
			ContentType: mime.TypeByExtension(path.Ext(resourcePath)),
			ETag:        resourceETags[resourcePath],
		})
	}

	mux.Handle("/grpc-web-form.js", &resource{
		Data: webFormJS,
		ContentType: "text/javascript; charset=UTF-8",
		ETag:        computeETag(webFormJS),
	})
	mux.Handle("/grpc-web-form.css", &resource{
		Data: webFormCSS,
		ContentType: "text/css; charset=utf-8",
		ETag:        computeETag(webFormCSS),
	})

	indexContents := getIndexContents(target, webFormHTML)
	indexResource := &resource{
		Data: indexContents,
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

var indexTemplate = template.Must(template.New("index.html").Parse(string(standalone.MustAsset("index-template.html"))))

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
	Data        []byte
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
	w.Write(res.Data)
}

var resourceETags = map[string]string{}

func init() {
	for _, assetName := range standalone.AssetNames() {
		resourcePath := "/" + assetName
		resourceETags[resourcePath] = computeETag(standalone.MustAsset(assetName))
	}
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
func HandlerViaReflection(ctx context.Context, cc *grpc.ClientConn, target string) (http.Handler, error) {
	m, err := grpcui.AllMethodsViaReflection(ctx, cc)
	if err != nil {
		return nil, err
	}

	f, err := grpcui.AllFilesViaReflection(ctx, cc)
	if err != nil {
		return nil, err
	}

	return Handler(cc, target, m, f), nil
}
