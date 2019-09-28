package grpcui

import (
	"bytes"
	"html/template"
	"os"
	"sort"

	"github.com/jhump/protoreflect/desc"

	"github.com/fullstorydev/grpcui/internal/resources/webform"
)

var webFormTemplate = template.Must(template.New("grpc web form").Parse(string(webform.Template())))

// WebFormContents returns an HTML form that can be embedded into a web UI to
// provide an interactive form for issuing RPCs.
//
// For a fully self-contained handler that provides both an HTML UI and the
// needed server handlers, see grpcui.UIHandler instead.
//
// The given invokeURI and metadataURI indicate the URI paths where server
// handlers are registered for invoking RPCs and querying RPC metadata,
// respectively. Handlers for these endpoints are provided via the
// RPCInvokeHandler and RPCMetadataHandler functions:
//
//   // This example uses "/rpcs" as the base URI.
//   pageHandler := func(w http.ResponseWriter, r *http.Request) {
//     webForm := grpcui.WebFormContents("/rpcs/invoke/", "/rpcs/metadata", descs)
//     webFormJs := grpcui.WebFormScript()
//     generateHTMLPage(w, r, webForm, webFormJs)
//   }
//
//   // Make sure the RPC handlers are registered at the same URI paths
//   // that were used in the call to WebFormContents:
//   rpcInvokeHandler := http.StripPrefix("/rpcs/invoke", grpcui.RPCInvokeHandler(conn, descs))
//   mux.Handle("/rpcs/invoke/", rpcInvokeHandler)
//   mux.Handle("/rpcs/metadata", grpcui.RPCMetadataHandler(descs))
//   mux.HandleFunc("/rpcs/index.html", pageHandler)
//
// The given descs is a slice of methods which are exposed through the web form.
// You can use AllMethodsForServices, AllMethodsForServer, and
// AllMethodsViaReflection helper functions to build this list.
//
// The returned HTML form requires that the contents of WebFormScript() have
// already been loaded as a script in the page.
func WebFormContents(invokeURI, metadataURI string, descs []*desc.MethodDescriptor) []byte {
	params := struct {
		InvokeURI   string
		MetadataURI string
		Services    []string
		Methods     map[string][]string
		Debug       bool
	}{
		InvokeURI:   invokeURI,
		MetadataURI: metadataURI,
		Methods:     map[string][]string{},
		// TODO(jh): parameter for enabling this instead of env var?
		Debug: os.Getenv("GRPC_WEBFORM_DEBUG") != "",
	}

	// build list of distinct service and method names and sort them
	uniqueServices := map[string]struct{}{}
	for _, md := range descs {
		svcName := md.GetService().GetFullyQualifiedName()
		uniqueServices[svcName] = struct{}{}
		params.Methods[svcName] = append(params.Methods[svcName], md.GetName())
	}
	for svcName := range uniqueServices {
		params.Services = append(params.Services, svcName)
	}
	sort.Strings(params.Services)
	for _, methods := range params.Methods {
		sort.Strings(methods)
	}

	// render the template
	var formBuf bytes.Buffer
	if err := webFormTemplate.Execute(&formBuf, params); err != nil {
		panic(err)
	}
	return formBuf.Bytes()
}

// WebFormScript returns the JavaScript that powers the web form returned by
// WebFormContents.
//
// The returned JavaScript requires that jQuery and jQuery UI libraries already
// be loaded in the container HTML page. It includes JavaScript code that relies
// on the "$" symbol.
//
// Note that the script, by default, does not handle CSRF protection. To add
// that, the enclosing page, in which the script is embedded, can use jQuery to
// configure this. For example, you can use the $.ajaxSend() jQuery function to
// intercept RPC invocations and automatically add a CSRF token header. To then
// check the token on the server-side, you will need to create a wrapper handler
// that first verifies the CSRF token header before delegating to a
// RPCInvokeHandler.
func WebFormScript() []byte {
	return webform.Script()
}

// WebFormSampleCSS returns a CSS stylesheet for styling the HTML web form
// returned by WebFormContents. It is possible for uses of the web form to
// supply their own stylesheet, but this makes it simple to use default
// styling.
func WebFormSampleCSS() []byte {
	return webform.SampleCSS()
}
