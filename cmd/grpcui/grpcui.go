// Command grpcui starts a simple web server that provides a web form for making gRPC requests.
// Command line parameters control how grpcui connects to the gRPC backend which actually services
// the requests. It can use a supplied descriptor file, proto source files, or service reflection
// for discovering the schema to expose.
package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"math"
	"mime"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fullstorydev/grpcurl"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/grpcreflect"
	"github.com/pkg/browser"
	"golang.org/x/term"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	insecurecreds "google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"

	// Register gzip compressor so compressed responses will work
	_ "google.golang.org/grpc/encoding/gzip"
	// Register xds so xds and xds-experimental resolver schemes work
	_ "google.golang.org/grpc/xds"

	"github.com/fullstorydev/grpcui/internal"
	"github.com/fullstorydev/grpcui/standalone"
)

var version = "dev build <no version set>"

var (
	grpcCurlFlags = []string{
		"key",
		"cert",
		"cacert",
		"plaintext",
		"insecure",
	}
)

var (
	exit = os.Exit

	isUnixSocket func() bool // nil when run on non-unix platform

	flags = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	help = flags.Bool("help", false, prettify(`
		Print usage instructions and exit.`))
	printVersion = flags.Bool("version", false, prettify(`
		Print version.`))
	plaintext = flags.Bool("plaintext", false, prettify(`
		Use plain-text HTTP/2 when connecting to server (no TLS).`))
	insecure = flags.Bool("insecure", false, prettify(`
		Skip server certificate and domain verification. (NOT SECURE!) Not
		valid with -plaintext option.`))
	cacert = flags.String("cacert", "", prettify(`
		File containing trusted root certificates for verifying the server.
		Ignored if -insecure is specified.`))
	cert = flags.String("cert", "", prettify(`
		File containing client certificate (public key), to present to the
		server. Not valid with -plaintext option. Must also provide -key option.`))
	key = flags.String("key", "", prettify(`
		File containing client private key, to present to the server. Not valid
		with -plaintext option. Must also provide -cert option.`))
	protoset      multiString
	protoFiles    multiString
	importPaths   multiString
	addlHeaders   multiString
	rpcHeaders    multiString
	reflHeaders   multiString
	prsvHeaders   multiString
	defHeaders    multiString
	expandHeaders = flags.Bool("expand-headers", false, prettify(`
		If set, headers may use '${NAME}' syntax to reference environment
		variables. These will be expanded to the actual environment variable
		value before sending to the server. For example, if there is an
		environment variable defined like FOO=bar, then a header of
		'key: ${FOO}' would expand to 'key: bar'. This applies to -H,
		-rpc-header, -reflect-header, and -default-header options. No other
		expansion/escaping is performed. This can be used to supply
		credentials/secrets without having to put them in command-line arguments.`))
	authority = flags.String("authority", "", prettify(`
		The authoritative name of the remote server. This value is passed as the
		value of the ":authority" pseudo-header in the HTTP/2 protocol. When TLS
		is used, this will also be used as the server name when verifying the
		server's certificate. It defaults to the address that is provided in the
		positional arguments.`))
	connectTimeout = flags.Float64("connect-timeout", 0, prettify(`
		The maximum time, in seconds, to wait for connection to be established.
		Defaults to 10 seconds.`))
	connectFailFast = flags.Bool("connect-fail-fast", true, prettify(`
		If true, non-temporary errors (such as "connection refused" during
		initial connection will cause the program to immediately abort. This
		is the default and is appropriate for interactive uses of grpcui. But
		long-lived server use (like as a sidecar to a gRPC server) will prefer
		to set this to false for more robust startup.`))
	keepaliveTime = flags.Float64("keepalive-time", 0, prettify(`
		If present, the maximum idle time in seconds, after which a keepalive
		probe is sent. If the connection remains idle and no keepalive response
		is received for this same period then the connection is closed and the
		operation fails.`))
	maxTime = flags.Float64("max-time", 0, prettify(`
		The maximum total time a single RPC invocation is allowed to take, in
		seconds.`))
	maxMsgSz = flags.Int("max-msg-sz", 0, prettify(`
		The maximum encoded size of a message that grpcui will accept. If not
		specified, defaults to 4mb.`))
	emitDefaults = flags.Bool("emit-defaults", true, prettify(`
		Emit default values for JSON-encoded responses.`))
	debug   optionalBoolFlag
	verbose = flags.Bool("v", false, prettify(`
		Enable verbose output.`))
	veryVerbose = flags.Bool("vv", false, prettify(`
		Enable *very* verbose output.`))
	veryVeryVerbose = flags.Bool("vvv", false, prettify(`
		Enable the most verbose output, which includes traces of all HTTP
		requests and responses.`))
	serverName = flags.String("servername", "", prettify(`
		Override server name when validating TLS certificate. This flag is
		ignored if -plaintext or -insecure is used.
		NOTE: Prefer -authority. This flag may be removed in the future. It is
		an error to use both -authority and -servername (though this will be
		permitted if they are both set to the same value, to increase backwards
		compatibility with earlier releases that allowed both to be set).`))
	openBrowser = flags.Bool("open-browser", false, prettify(`
		When true, grpcui will try to open a browser pointed at the UI's URL.
		This defaults to true when grpcui is used in an interactive mode; e.g.
		when the tool detects that stdin is a terminal/tty. Otherwise, this
		defaults to false.`))
	examplesFile = flags.String("examples", "", prettify(`
		Load examples from the given JSON file. The examples are shown in the UI
		which lets users pick pre-defined RPC and request data, like a recipe.
		This can be templates for common requests or could even represent a test
		suite as a sequence of RPCs. This is similar to the "collections" feature
		in postman. The format of this file is the same as used when saving
		history from the gRPC UI "History" tab.`))
	reflection = optionalBoolFlag{val: true}

	port = flags.Int("port", 0, prettify(`
		The port on which the web UI is exposed.`))
	bind = flags.String("bind", "127.0.0.1", prettify(`
		The address on which the web UI is exposed.`))
	basePath = flags.String("base-path", "/", prettify(`
		The path on which the web UI is exposed.
		Defaults to slash ("/"), which is the root of the server.
		Example: "/debug/grpcui".`))
	services multiString
	methods  multiString

	extraJS     multiString
	extraCSS    multiString
	otherAssets multiString
)

func init() {
	flags.Var(&addlHeaders, "H", prettify(`
		Additional headers in 'name: value' format. May specify more than one
		via multiple flags. These headers will also be included in reflection
		requests to a server. These headers are not shown in the gRPC UI form.
		If the user enters conflicting metadata, the user-entered value will be
		ignored and only the values present on the command-line will be used.`))
	flags.Var(&rpcHeaders, "rpc-header", prettify(`
		Additional RPC headers in 'name: value' format. May specify more than
		one via multiple flags. These headers will *only* be used when invoking
		the requested RPC method. They are excluded from reflection requests.
		These headers are not shown in the gRPC UI form. If the user enters
		conflicting metadata, the user-entered value will be ignored and only
		the values present on the command-line will be used.`))
	flags.Var(&reflHeaders, "reflect-header", prettify(`
		Additional reflection headers in 'name: value' format. May specify more
		than one via multiple flags. These headers will only be used during
		reflection requests and will be excluded when invoking the requested RPC
		method.`))
	flags.Var(&prsvHeaders, "preserve-header", prettify(`
		Header names (no values) for request headers that should be preserved
		when making requests to the gRPC server. May specify more than one via
		multiple flags. In addition to the headers given in -H and -rpc-header
		flags and metadata defined in the gRPC UI form, the gRPC server will also
		get any of the named headers that the gRPC UI web server receives as HTTP
		request headers. This can be useful if gRPC UI is behind an
		authenticating proxy, for example, that adds JWTs to HTTP requests.
		Having gRPC UI preserve these headers means that the JWTs will also be
		sent to backend gRPC servers. These headers are only sent when RPCs are
		invoked and are not included for reflection requests.`))
	flags.Var(&defHeaders, "default-header", prettify(`
		Additional headers to add to metadata in the gRPCui web form. Each value
		should be in 'name: value' format. May specify more than one via multiple
		flags. These headers are just defaults in the UI and maybe changed or
		removed by the user that is interacting with the form.`))
	flags.Var(&protoset, "protoset", prettify(`
		The name of a file containing an encoded FileDescriptorSet. This file's
		contents will be used to determine the RPC schema instead of querying
		for it from the remote server via the gRPC reflection API. May specify
		more than one via multiple -protoset flags. It is an error to use both
		-protoset and -proto flags.`))
	flags.Var(&protoFiles, "proto", prettify(`
		The name of a proto source file. Source files given will be used to
		determine the RPC schema instead of querying for it from the remote
		server via the gRPC reflection API. May specify more than one via
		multiple -proto flags. Imports will be resolved using the given
		-import-path flags. Multiple proto files can be specified by specifying
		multiple -proto flags. It is an error to use both -protoset and -proto
		flags.`))
	flags.Var(&importPaths, "import-path", prettify(`
		The path to a directory from which proto sources can be imported,
		for use with -proto flags. Multiple import paths can be configured by
		specifying multiple -import-path flags. Paths will be searched in the
		order given. If no import paths are given, all files (including all
		imports) must be provided as -proto flags, and grpcui will attempt to
		resolve all import statements from the set of file names given.`))
	flags.Var(&services, "service", prettify(`
		The services to expose through the web UI. If no -service and no -method
		flags are given, the web UI will expose *all* services and methods found
		(either via server reflection or in the given proto source or protoset
		files). If present, the methods exposed in the web UI include all
		methods for services named in a -service flag plus all methods named in
		a -method flag. Service names must be fully-qualified.`))
	flags.Var(&methods, "method", prettify(`
		The methods to expose through the web UI. If no -service and no -method
		flags are given, the web UI will expose *all* services and methods found
		(either via server reflection or in the given proto source or protoset
		files). If present, the methods exposed in the web UI include all
		methods for services named in a -service flag plus all methods named in
		a -method flag. Method names must be fully-qualified and may either use
		a dot (".") or a slash ("/") to separate the fully-qualified service
		name from the method's name.`))
	flags.Var(&debug, "debug-client", prettify(`
		When true, the client JS code in the gRPCui web form will log extra
		debug info to the console.`))
	flags.Var(&reflection, "use-reflection", prettify(`
		When true, server reflection will be used to determine the RPC schema.
		Defaults to true unless a -proto or -protoset option is provided. If
		-use-reflection is used in combination with a -proto or -protoset flag,
		the provided descriptor sources will be used in addition to server
		reflection to resolve messages and extensions.`))
	flags.Var(&extraJS, "extra-js", prettify(`
		Indicates the name of a JavaScript file to load from the web form. This
		allows injecting custom behavior into the page. Multiple files can be
		added by specifying multiple -extra-js flags.`))
	flags.Var(&extraCSS, "extra-css", prettify(`
		Indicates the name of a CSS file to load from the web form. This allows
		injecting custom styles into the page, to customize the look. Multiple
		files can be added by specifying multiple -extra-css flags.`))
	flags.Var(&otherAssets, "also-serve", prettify(`
		Indicates the name of an additional file or folder that the gRPC UI web
		server can serve. This can be useful for serving other assets, like
		images, when a custom CSS is used via -extra-css flags. Multiple assets
		can be added to the server by specifying multiple -also-serve flags. The
		named file will be available at a URI of "/<base-name>", where
		<base-name> is the name of the file, excluding its path. If the given
		name is a folder, the folder's contents are available at URIs that are
		under "/<base-name>/". It is an error to specify multiple files or
		folders that have the same base name.`))
}

type multiString []string

func (s *multiString) String() string {
	return strings.Join(*s, ",")
}

func (s *multiString) Set(value string) error {
	*s = append(*s, value)
	return nil
}

type optionalBoolFlag struct {
	set, val bool
}

func (f *optionalBoolFlag) String() string {
	if !f.set {
		return "unset"
	}
	return strconv.FormatBool(f.val)
}

func (f *optionalBoolFlag) Set(s string) error {
	v, err := strconv.ParseBool(s)
	if err != nil {
		return err
	}
	f.set = true
	f.val = v
	return nil
}

func (f *optionalBoolFlag) IsBoolFlag() bool {
	return true
}

// Uses a file source as a fallback for resolving symbols and extensions, but
// only uses the reflection source for listing services
type compositeSource struct {
	reflection grpcurl.DescriptorSource
	file       grpcurl.DescriptorSource
}

func (cs compositeSource) ListServices() ([]string, error) {
	return cs.reflection.ListServices()
}

func (cs compositeSource) FindSymbol(fullyQualifiedName string) (desc.Descriptor, error) {
	d, err := cs.reflection.FindSymbol(fullyQualifiedName)
	if err == nil {
		return d, nil
	}
	return cs.file.FindSymbol(fullyQualifiedName)
}

func (cs compositeSource) AllExtensionsForType(typeName string) ([]*desc.FieldDescriptor, error) {
	exts, err := cs.reflection.AllExtensionsForType(typeName)
	if err != nil {
		// On error fall back to file source
		return cs.file.AllExtensionsForType(typeName)
	}
	// Track the tag numbers from the reflection source
	tags := make(map[int32]bool)
	for _, ext := range exts {
		tags[ext.GetNumber()] = true
	}
	fileExts, err := cs.file.AllExtensionsForType(typeName)
	if err != nil {
		return exts, nil
	}
	for _, ext := range fileExts {
		// Prioritize extensions found via reflection
		if !tags[ext.GetNumber()] {
			exts = append(exts, ext)
		}
	}
	return exts, nil
}

func main() {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		*openBrowser = true
	}

	flags.Usage = usage
	flags.Parse(os.Args[1:])

	var gRPCOptions []string
	for _, flagName := range grpcCurlFlags {
		f := flags.Lookup(flagName)
		if f.Value.String() != f.DefValue {
			if getter, ok := f.Value.(flag.Getter); ok && getter.Get() == true {
				gRPCOptions = append(gRPCOptions, fmt.Sprintf("-%s", f.Name))
			} else {
				gRPCOptions = append(gRPCOptions, fmt.Sprintf("-%s=%s", f.Name, strconv.Quote(f.Value.String())))
			}
		}
	}

	if *help {
		usage()
		os.Exit(0)
	}
	if *printVersion {
		fmt.Fprintf(os.Stderr, "%s %s\n", os.Args[0], version)
		os.Exit(0)
	}

	// Do extra validation on arguments and figure out what user asked us to do.
	if *connectTimeout < 0 {
		fail(nil, "The -connect-timeout argument must not be negative.")
	}
	if *keepaliveTime < 0 {
		fail(nil, "The -keepalive-time argument must not be negative.")
	}
	if *maxTime < 0 {
		fail(nil, "The -max-time argument must not be negative.")
	}
	if *maxMsgSz < 0 {
		fail(nil, "The -max-msg-sz argument must not be negative.")
	}
	if *plaintext && *insecure {
		fail(nil, "The -plaintext and -insecure arguments are mutually exclusive.")
	}
	if *plaintext && *cert != "" {
		fail(nil, "The -plaintext and -cert arguments are mutually exclusive.")
	}
	if *plaintext && *key != "" {
		fail(nil, "The -plaintext and -key arguments are mutually exclusive.")
	}
	if (*key == "") != (*cert == "") {
		fail(nil, "The -cert and -key arguments must be used together and both be present.")
	}

	if flags.NArg() != 1 {
		fail(nil, "This program requires exactly one arg: the host:port of gRPC server.")
	}
	target := flags.Arg(0)

	if len(protoset) > 0 && len(reflHeaders) > 0 {
		warn("The -reflect-header argument is not used when -protoset files are used.")
	}
	if len(protoset) > 0 && len(protoFiles) > 0 {
		fail(nil, "Use either -protoset files or -proto files, but not both.")
	}
	if len(importPaths) > 0 && len(protoFiles) == 0 {
		warn("The -import-path argument is not used unless -proto files are used.")
	}
	if !reflection.val && len(protoset) == 0 && len(protoFiles) == 0 {
		fail(nil, "No protoset files or proto files specified and -use-reflection set to false.")
	}
	if !strings.HasPrefix(*basePath, "/") {
		fail(nil, `The -base-path must begin with a slash ("/")`)
	}

	assetNames := map[string]string{}
	checkAssetNames(assetNames, extraJS, true)
	checkAssetNames(assetNames, extraCSS, true)
	checkAssetNames(assetNames, otherAssets, false)

	// Protoset or protofiles provided and -use-reflection unset
	if !reflection.set && (len(protoset) > 0 || len(protoFiles) > 0) {
		reflection.val = false
	}

	configs, err := computeSvcConfigs()
	if err != nil {
		fail(err, "Invalid services/methods indicated")
	}

	var verbosity int
	switch {
	case *veryVeryVerbose:
		verbosity = 3
	case *veryVerbose:
		verbosity = 2
	case *verbose:
		verbosity = 1
	}

	if verbosity == 1 {
		// verbose will let grpc package print warnings and errors
		grpclog.SetLoggerV2(grpclog.NewLoggerV2(io.Discard, os.Stdout, io.Discard))
	} else if verbosity > 1 {
		// very verbose will let grpc package log info
		// and very very verbose turns up the verbosity
		grpcVerbosity := 0
		if verbosity > 2 {
			grpcVerbosity = 5
		}
		grpclog.SetLoggerV2(grpclog.NewLoggerV2WithVerbosity(os.Stdout, io.Discard, io.Discard, grpcVerbosity))
	}

	var examplesOpt standalone.HandlerOption
	if *examplesFile != "" {
		func() {
			f, err := os.Open(*examplesFile)
			if err != nil {
				if os.IsNotExist(err) {
					fail(nil, "File %q does not exist", *examplesFile)
				} else {
					fail(err, "Failed to open %q", *examplesFile)
				}
			}
			defer func() {
				_ = f.Close()
			}()
			data, err := io.ReadAll(f)
			if err != nil {
				fail(err, "Failed to read contents of %q", *examplesFile)
			}
			examplesOpt, err = standalone.WithExampleData(data)
			if err != nil {
				fail(err, "Failed to process contents of %q", *examplesFile)
			}
		}()
	}

	ctx := context.Background()
	dialTime := 10 * time.Second
	if *connectTimeout > 0 {
		dialTime = floatSecondsToDuration(*connectTimeout)
	}
	dialCtx, cancel := context.WithTimeout(ctx, dialTime)
	defer cancel()
	var opts []grpc.DialOption
	if *keepaliveTime > 0 {
		timeout := floatSecondsToDuration(*keepaliveTime)
		opts = append(opts, grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:    timeout,
			Timeout: timeout,
		}))
	}
	if *maxMsgSz > 0 {
		opts = append(opts, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(*maxMsgSz)))
	}

	if *expandHeaders {
		var err error
		addlHeaders, err = grpcurl.ExpandHeaders(addlHeaders)
		if err != nil {
			fail(err, "Failed to expand additional headers")
		}
		rpcHeaders, err = grpcurl.ExpandHeaders(rpcHeaders)
		if err != nil {
			fail(err, "Failed to expand rpc headers")
		}
		reflHeaders, err = grpcurl.ExpandHeaders(reflHeaders)
		if err != nil {
			fail(err, "Failed to expand reflection headers")
		}
		defHeaders, err = grpcurl.ExpandHeaders(defHeaders)
		if err != nil {
			fail(err, "Failed to expand default headers")
		}
	}

	var creds credentials.TransportCredentials
	if !*plaintext {
		tlsConf, err := grpcurl.ClientTLSConfig(*insecure, *cacert, *cert, *key)
		if err != nil {
			fail(err, "Failed to create TLS config")
		}
		creds = credentials.NewTLS(tlsConf)

		// can use either -servername or -authority; but not both
		if *serverName != "" && *authority != "" {
			if *serverName == *authority {
				warn("Both -servername and -authority are present; prefer only -authority.")
			} else {
				fail(nil, "Cannot specify different values for -servername and -authority.")
			}
		}
		overrideName := *serverName
		if overrideName == "" {
			overrideName = *authority
		}

		if overrideName != "" {
			opts = append(opts, grpc.WithAuthority(overrideName))
		}
	} else if *authority != "" {
		opts = append(opts, grpc.WithAuthority(*authority))
	}
	network := "tcp"
	if isUnixSocket != nil && isUnixSocket() {
		network = "unix"
	}
	cc, err := dial(dialCtx, network, target, creds, *connectFailFast, opts...)
	if err != nil {
		fail(err, "Failed to dial target host %q", target)
	}

	var descSource grpcurl.DescriptorSource
	var refClient *grpcreflect.Client
	var fileSource grpcurl.DescriptorSource
	if len(protoset) > 0 {
		var err error
		fileSource, err = grpcurl.DescriptorSourceFromProtoSets(protoset...)
		if err != nil {
			fail(err, "Failed to process proto descriptor sets.")
		}
	} else if len(protoFiles) > 0 {
		var err error
		fileSource, err = grpcurl.DescriptorSourceFromProtoFiles(importPaths, protoFiles...)
		if err != nil {
			fail(err, "Failed to process proto source files.")
		}
	}
	if reflection.val {
		md := grpcurl.MetadataFromHeaders(append(addlHeaders, reflHeaders...))
		refCtx := metadata.NewOutgoingContext(ctx, md)
		refClient = grpcreflect.NewClientAuto(refCtx, cc)
		refClient.AllowMissingFileDescriptors()
		reflSource := grpcurl.DescriptorSourceFromServer(ctx, refClient)
		if fileSource != nil {
			descSource = compositeSource{reflSource, fileSource}
		} else {
			descSource = reflSource
		}
	} else {
		descSource = fileSource
	}

	// arrange for the RPCs to be cleanly shutdown
	reset := func() {
		if refClient != nil {
			refClient.Reset()
			refClient = nil
		}
		if cc != nil {
			cc.Close()
			cc = nil
		}
	}
	defer reset()
	exit = func(code int) {
		// since defers aren't run by os.Exit...
		reset()
		os.Exit(code)
	}

	methods, err := getMethods(descSource, configs)
	if err != nil {
		fail(err, "Failed to compute set of methods to expose")
	}
	allFiles, err := grpcurl.GetAllFiles(descSource)
	if err != nil {
		fail(err, "Failed to enumerate all proto files")
	}

	// can go ahead and close reflection client now
	if refClient != nil {
		refClient.Reset()
		refClient = nil
	}

	var handlerOpts []standalone.HandlerOption
	if len(defHeaders) > 0 {
		handlerOpts = append(handlerOpts, standalone.WithDefaultMetadata(defHeaders))
	}
	if len(addlHeaders) > 0 || len(rpcHeaders) > 0 {
		handlerOpts = append(handlerOpts, standalone.WithMetadata(append(addlHeaders, rpcHeaders...)))
	}
	if len(prsvHeaders) > 0 {
		handlerOpts = append(handlerOpts, standalone.PreserveHeaders(prsvHeaders))
	}
	if verbosity > 0 {
		handlerOpts = append(handlerOpts, standalone.WithInvokeVerbosity(verbosity))
	}
	if debug.set {
		handlerOpts = append(handlerOpts, standalone.WithClientDebug(debug.val))
	}
	if examplesOpt != nil {
		handlerOpts = append(handlerOpts, examplesOpt)
	}
	handlerOpts = append(handlerOpts, standalone.EmitDefaults(*emitDefaults))
	handlerOpts = append(handlerOpts, configureJSandCSS(extraJS, standalone.AddJSFile)...)
	handlerOpts = append(handlerOpts, configureJSandCSS(extraCSS, standalone.AddCSSFile)...)
	handlerOpts = append(handlerOpts, configureAssets(otherAssets)...)
	handlerOpts = append(handlerOpts, standalone.WithGRPCOptions(gRPCOptions))

	handler := standalone.Handler(cc, target, methods, allFiles, handlerOpts...)
	if *maxTime > 0 {
		timeout := floatSecondsToDuration(*maxTime)
		// enforce the timeout by wrapping the handler and inserting a
		// context timeout for invocation calls
		orig := handler
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/invoke/") {
				ctx, cancel := context.WithTimeout(r.Context(), timeout)
				defer cancel()
				r = r.WithContext(ctx)
			}
			orig.ServeHTTP(w, r)
		})
	}

	if verbosity > 0 {
		// wrap the handler with one that performs more logging of what's going on
		orig := handler
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			if verbosity > 1 {
				// TODO: the web form never sends binary request body; but maybe a custom
				//  JS addition could, so maybe this should be a custom printer of the request
				//  like we have for the response below?
				if req, err := httputil.DumpRequest(r, verbosity > 2); err != nil {
					internal.LogErrorf("could not dump request: %v", err)
				} else {
					internal.LogInfof("received request:\n%s", string(req))
				}
				var recorder httptest.ResponseRecorder
				if verbosity > 2 {
					recorder.Body = &bytes.Buffer{}
				}
				tee := teeWriter{w: []http.ResponseWriter{w, &recorder}}
				w = &tee
				defer func() {
					// in case handler never called Write or WriteHeader:
					tee.mirrorHeaders()

					if resp, err := dumpResponse(recorder.Result(), verbosity > 2); err != nil {
						internal.LogErrorf("could not dump response: %v", err)
					} else {
						internal.LogInfof("sent response:\n%s", resp)
					}
				}()
			}

			cs := codeSniffer{w: w}
			orig.ServeHTTP(&cs, r)

			millis := time.Since(start).Nanoseconds() / (1000 * 1000)
			internal.LogInfof("%s %s %s %d %dms %dbytes", r.RemoteAddr, r.Method, r.RequestURI, cs.code, millis, cs.size)
		})
	}
	if *basePath != "/" {
		withoutSlash := strings.TrimSuffix(*basePath, "/")
		mux := http.NewServeMux()
		// the mux will correctly redirect the bare path (without trailing slash)
		mux.Handle(withoutSlash+"/", http.StripPrefix(withoutSlash, handler))
		handler = mux
	}

	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", *bind, *port))
	if err != nil {
		fail(err, "Failed to listen on port %d", *port)
	}

	path := *basePath
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	url := fmt.Sprintf("http://%s:%d%s", *bind, listener.Addr().(*net.TCPAddr).Port, path)
	fmt.Printf("gRPC Web UI available at %s\n", url)

	if *openBrowser {
		go func() {
			if err := browser.OpenURL(url); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to open browser: %v\n", err)
			}
		}()
	}
	if err := http.Serve(listener, handler); err != nil {
		fail(err, "Failed to serve web UI")
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `Usage:
	%s [flags] [address]

Starts a web server that hosts a web UI for sending RPCs to the given address.

The address will typically be in the form "host:port" where host can be an IP
address or a hostname and port is a numeric port or service name. If an IPv6
address is given, it must be surrounded by brackets, like "[2001:db8::1]". For
Unix variants, if a -unix=true flag is present, then the address must be the
path to the domain socket.

Most flags control how the connection to the gRPC server is established. The
web server will always bind only to localhost, without TLS, so only the port
can be controlled via command-line flags.

Available flags:
`, os.Args[0])
	flags.PrintDefaults()
}

func prettify(docString string) string {
	parts := strings.Split(docString, "\n")

	// cull empty lines and also remove trailing and leading spaces
	// from each line in the doc string
	j := 0
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		parts[j] = part
		j++
	}

	return strings.Join(parts[:j], "\n")
}

func warn(msg string, args ...interface{}) {
	msg = fmt.Sprintf("Warning: %s\n", msg)
	fmt.Fprintf(os.Stderr, msg, args...)
}

func fail(err error, msg string, args ...interface{}) {
	if err != nil {
		msg += ": %v"
		args = append(args, err)
	}
	fmt.Fprintf(os.Stderr, msg, args...)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		exit(1)
	} else {
		// nil error means it was CLI usage issue
		fmt.Fprintf(os.Stderr, "Try '%s -help' for more details.\n", os.Args[0])
		exit(2)
	}
}

func checkAssetNames(soFar map[string]string, names []string, requireFile bool) {
	for _, n := range names {
		st, err := os.Stat(n)
		if err != nil {
			if os.IsNotExist(err) {
				fail(nil, "File %q does not exist", n)
			} else {
				fail(err, "Failed to check existence of file %q", n)
			}
		}
		if requireFile && st.IsDir() {
			fail(nil, "Path %q is a folder, not a file", n)
		}

		base := filepath.Base(n)
		if existing, ok := soFar[base]; ok {
			fail(nil, "Multiple assets with the same base name specified: %s and %s", existing, n)
		}
		soFar[base] = n
	}
}

func configureJSandCSS(names []string, fn func(string, func() (io.ReadCloser, error)) standalone.HandlerOption) []standalone.HandlerOption {
	opts := make([]standalone.HandlerOption, len(names))
	for i := range names {
		name := names[i] // no loop variable so that we don't close over loop var in lambda below
		open := func() (io.ReadCloser, error) {
			return os.Open(name)
		}
		opts[i] = fn(filepath.Base(name), open)
	}
	return opts
}

func configureAssets(names []string) []standalone.HandlerOption {
	opts := make([]standalone.HandlerOption, len(names))
	for i := range names {
		name := names[i] // no loop variable so that we don't close over loop var in lambdas below
		st, err := os.Stat(name)
		if err != nil {
			fail(err, "failed to inspect file %q", name)
		}
		if st.IsDir() {
			open := func(p string) (io.ReadCloser, error) {
				path := filepath.Join(name, p)
				st, err := os.Stat(path)
				if err == nil && st.IsDir() {
					// Strangely, os.Open does not return an error if given a directory
					// and instead returns an empty reader :(
					// So check that first and return a 404 if user indicates directory name
					return nil, os.ErrNotExist
				}
				return os.Open(path)
			}
			opts[i] = standalone.ServeAssetDirectory(filepath.Base(name), open)
		} else {
			open := func() (io.ReadCloser, error) {
				return os.Open(name)
			}
			opts[i] = standalone.ServeAssetFile(filepath.Base(name), open)
		}
	}
	return opts
}

type svcConfig struct {
	includeService bool
	includeMethods map[string]struct{}
}

func getMethods(source grpcurl.DescriptorSource, configs map[string]*svcConfig) ([]*desc.MethodDescriptor, error) {
	servicesConfigured := len(configs) > 0
	allServices, err := source.ListServices()
	if err != nil {
		return nil, err
	}

	var descs []*desc.MethodDescriptor
	for _, svc := range allServices {
		if svc == "grpc.reflection.v1alpha.ServerReflection" || svc == "grpc.reflection.v1.ServerReflection" {
			continue
		}
		d, err := source.FindSymbol(svc)
		if err != nil {
			return nil, err
		}
		sd, ok := d.(*desc.ServiceDescriptor)
		if !ok {
			return nil, fmt.Errorf("%s should be a service descriptor but instead is a %T", d.GetFullyQualifiedName(), d)
		}
		cfg := configs[svc]
		if cfg == nil && servicesConfigured {
			// not configured to expose this service
			continue
		}
		delete(configs, svc)
		for _, md := range sd.GetMethods() {
			if cfg == nil {
				descs = append(descs, md)
				continue
			}
			_, found := cfg.includeMethods[md.GetName()]
			delete(cfg.includeMethods, md.GetName())
			if found && cfg.includeService {
				warn("Service %s already configured, so -method %s is unnecessary", svc, md.GetName())
			}
			if found || cfg.includeService {
				descs = append(descs, md)
			}
		}
		if cfg != nil && len(cfg.includeMethods) > 0 {
			// configured methods not found
			methodNames := make([]string, 0, len(cfg.includeMethods))
			for m := range cfg.includeMethods {
				methodNames = append(methodNames, fmt.Sprintf("%s/%s", svc, m))
			}
			sort.Strings(methodNames)
			return nil, fmt.Errorf("configured methods not found: %s", strings.Join(methodNames, ", "))
		}
	}

	if len(configs) > 0 {
		// configured services not found
		svcNames := make([]string, 0, len(configs))
		for s := range configs {
			svcNames = append(svcNames, s)
		}
		sort.Strings(svcNames)
		return nil, fmt.Errorf("configured services not found: %s", strings.Join(svcNames, ", "))
	}

	return descs, nil
}

func computeSvcConfigs() (map[string]*svcConfig, error) {
	if len(services) == 0 && len(methods) == 0 {
		return nil, nil
	}
	configs := map[string]*svcConfig{}
	for _, svc := range services {
		configs[svc] = &svcConfig{
			includeService: true,
			includeMethods: map[string]struct{}{},
		}
	}
	for _, fqMethod := range methods {
		svc, method := splitMethodName(fqMethod)
		if svc == "" || method == "" {
			return nil, fmt.Errorf("could not parse name into service and method names: %q", fqMethod)
		}
		cfg := configs[svc]
		if cfg == nil {
			cfg = &svcConfig{includeMethods: map[string]struct{}{}}
			configs[svc] = cfg
		}
		cfg.includeMethods[method] = struct{}{}
	}
	return configs, nil
}

func splitMethodName(name string) (svc, method string) {
	dot := strings.LastIndex(name, ".")
	slash := strings.LastIndex(name, "/")
	sep := dot
	if slash > dot {
		sep = slash
	}
	if sep < 0 {
		return "", name
	}
	return name[:sep], name[sep+1:]
}

type teeWriter struct {
	w           []http.ResponseWriter
	hdrsWritten bool
}

func (t *teeWriter) Header() http.Header {
	// we let callers modify the first set of headers, and then
	// we'll mirror them out later
	return t.w[0].Header()
}

func (t *teeWriter) Write(b []byte) (int, error) {
	t.mirrorHeaders()
	// treat first writer as authoritative (regarding return value)
	n, err := t.w[0].Write(b)
	for _, w := range t.w[1:] {
		w.Write(b)
	}
	return n, err
}

func (t *teeWriter) WriteHeader(statusCode int) {
	t.mirrorHeaders()
	for _, w := range t.w {
		w.WriteHeader(statusCode)
	}
}

func (t *teeWriter) mirrorHeaders() {
	if t.hdrsWritten {
		return
	}
	t.hdrsWritten = true

	source := t.w[0].Header()
	// copy hdr out to all other writers
	for _, w := range t.w[1:] {
		target := w.Header()
		// remove anything in target that doesn't belong
		for k := range target {
			if _, ok := source[k]; !ok {
				delete(target, k)
			}
		}
		// then copy over all values from source
		for k, v := range source {
			target[k] = v
		}
	}
}

type codeSniffer struct {
	w       http.ResponseWriter
	code    int
	codeSet bool
	size    int
}

func (cs *codeSniffer) Header() http.Header {
	return cs.w.Header()
}

func (cs *codeSniffer) Write(b []byte) (int, error) {
	if !cs.codeSet {
		cs.code = 200
		cs.codeSet = true
	}
	cs.size += len(b)
	return cs.w.Write(b)
}

func (cs *codeSniffer) WriteHeader(statusCode int) {
	if !cs.codeSet {
		cs.code = statusCode
		cs.codeSet = true
	}
	cs.w.WriteHeader(statusCode)
}

func dial(ctx context.Context, network, addr string, creds credentials.TransportCredentials, failFast bool, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	if failFast {
		return grpcurl.BlockingDial(ctx, network, addr, creds, opts...)
	}
	// BlockingDial will return the first error returned. It is meant for interactive use.
	// If we don't want to fail fast, then we need to do a more customized dial.

	// TODO: perhaps this logic should be added to the grpcurl package, like in a new
	// BlockingDialNoFailFast function?

	dialer := &errTrackingDialer{
		dialer:  &net.Dialer{},
		network: network,
	}
	var errCreds errTrackingCreds
	if creds == nil {
		opts = append(opts, grpc.WithTransportCredentials(insecurecreds.NewCredentials()))
	} else {
		errCreds = errTrackingCreds{
			TransportCredentials: creds,
		}
		opts = append(opts, grpc.WithTransportCredentials(&errCreds))
	}

	cc, err := grpc.DialContext(ctx, addr, append(opts, grpc.WithBlock(), grpc.WithContextDialer(dialer.dial))...)
	if err == nil {
		return cc, nil
	}

	// prefer last observed TLS handshake error if there is one
	if err := errCreds.err(); err != nil {
		return nil, err
	}
	// otherwise, use the error the dialer last observed
	if err := dialer.err(); err != nil {
		return nil, err
	}
	// if we have no better source of error message, use what grpc.DialContext returned
	return nil, err
}

type errTrackingCreds struct {
	credentials.TransportCredentials

	mu      sync.Mutex
	lastErr error
}

func (c *errTrackingCreds) ClientHandshake(ctx context.Context, addr string, rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	conn, auth, err := c.TransportCredentials.ClientHandshake(ctx, addr, rawConn)
	if err != nil {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.lastErr = err
	}
	return conn, auth, err
}

func (c *errTrackingCreds) err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastErr
}

type errTrackingDialer struct {
	dialer  *net.Dialer
	network string

	mu      sync.Mutex
	lastErr error
}

func (c *errTrackingDialer) dial(ctx context.Context, addr string) (net.Conn, error) {
	conn, err := c.dialer.DialContext(ctx, c.network, addr)
	if err != nil {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.lastErr = err
	}
	return conn, err
}

func (c *errTrackingDialer) err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastErr
}

func dumpResponse(r *http.Response, includeBody bool) (string, error) {
	// NB: not using httputil.DumpResponse because it writes binary data in the body which is
	//  not useful in the log (and can cause unexpected behavior when writing to a terminal,
	//  which may interpret some byte sequences as control codes).
	var buf bytes.Buffer
	buf.WriteString(r.Status)
	buf.WriteRune('\n')
	if err := r.Header.Write(&buf); err != nil {
		return "", err
	}
	if includeBody {
		buf.WriteRune('\n')
		ct := strings.ToLower(r.Header.Get("content-type"))
		mt, _, err := mime.ParseMediaType(ct)
		if err != nil {
			mt = ct
		}
		isText := strings.HasPrefix(mt, "text/") ||
			mt == "application/json" ||
			mt == "application/javascript" ||
			mt == "application/x-www-form-urlencoded" ||
			mt == "multipart/form-data" ||
			mt == "application/xml"
		if isText {
			if _, err := io.Copy(&buf, r.Body); err != nil {
				return "", err
			}
		} else {
			first := true
			var block [32]byte
			for {
				n, err := r.Body.Read(block[:])
				if n > 0 {
					if first {
						buf.WriteString("(binary body; encoded in hex)\n")
						first = false
					}
					for i := 0; i < n; i += 8 {
						end := i + 8
						if end > n {
							end = n
						}
						buf.WriteString(hex.EncodeToString(block[i:end]))
						buf.WriteRune(' ')
					}
					buf.WriteRune('\n')
				}
				if err == io.EOF {
					break
				} else if err != nil {
					return "", err
				}
			}
		}
	}
	return buf.String(), nil
}

func floatSecondsToDuration(seconds float64) time.Duration {
	durationFloat := seconds * float64(time.Second)
	if durationFloat > math.MaxInt64 {
		// Avoid overflow
		return math.MaxInt64
	}
	return time.Duration(durationFloat)
}
