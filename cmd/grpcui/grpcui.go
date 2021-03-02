// Command grpcui starts a simple web server that provides a web form for making gRPC requests.
// Command line parameters control how grpcui connects to the gRPC backend which actually services
// the requests. It can use a supplied descriptor file, proto source files, or service reflection
// for discovering the schema to expose.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fullstorydev/grpcurl"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/grpcreflect"
	"github.com/jpillora/backoff"
	"github.com/pkg/browser"
	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	reflectpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"

	// Register gzip compressor so compressed responses will work
	_ "google.golang.org/grpc/encoding/gzip"
	// Register xds so xds and xds-experimental resolver schemes work
	_ "google.golang.org/grpc/xds"

	"github.com/fullstorydev/grpcui/standalone"
)

var version = "dev build <no version set>"

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
	protoset    multiString
	protoFiles  multiString
	importPaths multiString
	reflHeaders multiString
	defHeaders  multiString
	authority   = flags.String("authority", "", prettify(`
		Value of :authority pseudo-header to be use with underlying HTTP/2
		requests. It defaults to the given address.`))
	connectTimeout = flags.Float64("connect-timeout", 0, prettify(`
		The maximum time, in seconds, to wait for connection to be established.
		Defaults to 10 seconds.`))
	keepaliveTime = flags.Float64("keepalive-time", 0, prettify(`
		If present, the maximum idle time in seconds, after which a keepalive
		probe is sent. If the connection remains idle and no keepalive response
		is received for this same period then the connection is closed and the
		operation fails.`))
	retryMaxAttempts = flags.Int("retry-max-attempts", 0, prettify(`
		The maximum number of times to attempt to reconnect to the gRPC server.
		grpcui will exponentially back off the time between connection attempts
		as it is unable to reach the server. Useful for running grpcui as a
		sidecar to gRPC service development. Defaults to 0.`))
	maxTime = flags.Float64("max-time", 0, prettify(`
		The maximum total time a single RPC invocation is allowed to take, in
		seconds.`))
	maxMsgSz = flags.Int("max-msg-sz", 0, prettify(`
		The maximum encoded size of a message that grpcui will accept. If not
		specified, defaults to 4mb.`))
	debug   optionalBoolFlag
	verbose = flags.Bool("v", false, prettify(`
		Enable verbose output.`))
	veryVerbose = flags.Bool("vv", false, prettify(`
		Enable *very* verbose output.`))
	veryVeryVerbose = flags.Bool("vvv", false, prettify(`
		Enable the most verbose output, which includes traces of all HTTP
		requests and responses.`))
	serverName = flags.String("servername", "", prettify(`
		Override servername when validating TLS certificate.`))
	openBrowser = flags.Bool("open-browser", false, prettify(`
		When true, grpcui will try to open a browser pointed at the UI's URL.
		This defaults to true when grpcui is used in an interactive mode; e.g.
		when the tool detects that stdin is a terminal/tty. Otherwise, this
		defaults to false.`))

	port = flags.Int("port", 0, prettify(`
		The port on which the web UI is exposed.`))
	bind = flags.String("bind", "127.0.0.1", prettify(`
		The address on which the web UI is exposed.`))
	services multiString
	methods  multiString
)

func init() {
	flags.Var(&reflHeaders, "reflect-header", prettify(`
		Additional reflection headers in 'name: value' format. May specify more
		than one via multiple flags. These headers will only be used during
		reflection requests and will be excluded when invoking the requested RPC
		method.`))
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

func main() {
	if terminal.IsTerminal(int(os.Stdin.Fd())) {
		*openBrowser = true
	}

	flags.Usage = usage
	flags.Parse(os.Args[1:])

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
	if *retryMaxAttempts < 0 {
		fail(nil, "The -retry-max-attempts argument must not be negative.")
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

	configs, err := computeSvcConfigs()
	if err != nil {
		fail(err, "Invalid services/methods indicated")
	}

	if *veryVeryVerbose {
		// very-very verbose implies very verbose
		*veryVerbose = true
	}

	if *verbose && !*veryVerbose {
		// verbose will let grpc package print warnings and errors
		grpclog.SetLoggerV2(grpclog.NewLoggerV2(ioutil.Discard, os.Stdout, ioutil.Discard))
	} else if *veryVerbose {
		// very verbose implies verbose
		*verbose = true

		// very verbose will let grpc package log info
		// and very very verbose turns up the verbosity
		v := 0
		if *veryVeryVerbose {
			v = 5
		}
		grpclog.SetLoggerV2(grpclog.NewLoggerV2WithVerbosity(os.Stdout, ioutil.Discard, ioutil.Discard, v))
	}

	ctx := context.Background()
	dialTime := 10 * time.Second
	if *connectTimeout > 0 {
		dialTime = time.Duration(*connectTimeout * float64(time.Second))
	}
	ctx, cancel := context.WithTimeout(ctx, dialTime)
	defer cancel()
	var opts []grpc.DialOption
	if *keepaliveTime > 0 {
		timeout := time.Duration(*keepaliveTime * float64(time.Second))
		opts = append(opts, grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:    timeout,
			Timeout: timeout,
		}))
	}
	if *maxMsgSz > 0 {
		opts = append(opts, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(*maxMsgSz)))
	}
	if *authority != "" {
		opts = append(opts, grpc.WithAuthority(*authority))
	}
	var creds credentials.TransportCredentials
	if !*plaintext {
		var err error
		creds, err = grpcurl.ClientTransportCredentials(*insecure, *cacert, *cert, *key)
		if err != nil {
			fail(err, "Failed to configure transport credentials")
		}
		if *serverName != "" {
			if err := creds.OverrideServerName(*serverName); err != nil {
				fail(err, "Failed to override server name as %q", *serverName)
			}
		}
	}
	network := "tcp"
	if isUnixSocket != nil && isUnixSocket() {
		network = "unix"
	}

	b := &backoff.Backoff{
		Min: 2 * time.Second,
		Max: 8 * time.Second,
	}
	retryCount := 0
	var cc *grpc.ClientConn
	for {
		if retryCount > 0 {
			fmt.Printf("Retry attempt %d of %d\n", retryCount, *retryMaxAttempts)
		}
		conn, err := grpcurl.BlockingDial(ctx, network, target, creds, opts...)
		if err != nil {
			if *retryMaxAttempts == 0 {
				fail(err, "Failed to reach host %q", target)
			} else {
				fmt.Printf("Unable to reach host %q\n", target)
				if retryCount == *retryMaxAttempts {
					fail(err, "Failed to reach host %q after %d attempts", target, retryCount)
				}
				d := b.Duration()
				fmt.Printf("Reconnecting in %s\n", d)
				time.Sleep(d)
				retryCount++
				continue
			}
		}
		// connected to the server
		cc = conn
		break
	}

	var descSource grpcurl.DescriptorSource
	var refClient *grpcreflect.Client
	if len(protoset) > 0 {
		var err error
		descSource, err = grpcurl.DescriptorSourceFromProtoSets(protoset...)
		if err != nil {
			fail(err, "Failed to process proto descriptor sets.")
		}
	} else if len(protoFiles) > 0 {
		var err error
		descSource, err = grpcurl.DescriptorSourceFromProtoFiles(importPaths, protoFiles...)
		if err != nil {
			fail(err, "Failed to process proto source files.")
		}
	} else {
		md := grpcurl.MetadataFromHeaders(reflHeaders)
		refCtx := metadata.NewOutgoingContext(ctx, md)
		refClient = grpcreflect.NewClient(refCtx, reflectpb.NewServerReflectionClient(cc))
		descSource = grpcurl.DescriptorSourceFromServer(ctx, refClient)
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
	if debug.set {
		handlerOpts = append(handlerOpts, standalone.WithDebug(debug.val))
	}
	handler := standalone.Handler(cc, target, methods, allFiles, handlerOpts...)
	if *maxTime > 0 {
		timeout := time.Duration(*maxTime * float64(time.Second))
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

	if *verbose {
		// wrap the handler with one that performs more logging of what's going on
		orig := handler
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			if *veryVerbose {
				if req, err := httputil.DumpRequest(r, *veryVeryVerbose); err != nil {
					logErrorf("could not dump request: %v", err)
				} else {
					logInfof("received request:\n%s", string(req))
				}
				var recorder httptest.ResponseRecorder
				if *veryVeryVerbose {
					recorder.Body = &bytes.Buffer{}
				}
				tee := teeWriter{w: []http.ResponseWriter{w, &recorder}}
				w = &tee
				defer func() {
					// in case handler never called Write or WriteHeader:
					tee.mirrorHeaders()

					if resp, err := httputil.DumpResponse(recorder.Result(), *veryVeryVerbose); err != nil {
						logErrorf("could not dump response: %v", err)
					} else {
						logInfof("sent response:\n%s", string(resp))
					}
				}()
			}

			cs := codeSniffer{w: w}
			orig.ServeHTTP(&cs, r)

			millis := time.Since(start).Nanoseconds() / (1000 * 1000)
			logInfof("%s %s %s %d %dms %dbytes", r.RemoteAddr, r.Method, r.RequestURI, cs.code, millis, cs.size)
		})
	}

	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", *bind, *port))
	if err != nil {
		fail(err, "Failed to listen on port %d", *port)
	}

	url := fmt.Sprintf("http://%s:%d/", *bind, listener.Addr().(*net.TCPAddr).Port)
	fmt.Printf("gRPC Web UI available at %s\n", url)

	if *openBrowser {
		if err := browser.OpenURL(url); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open browser: %v\n", err)
		}
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

	return strings.Join(parts[:j], "\n"+indent())
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

type svcConfig struct {
	includeService bool
	includeMethods map[string]struct{}
}

func getMethods(source grpcurl.DescriptorSource, configs map[string]*svcConfig) ([]*desc.MethodDescriptor, error) {
	allServices, err := source.ListServices()
	if err != nil {
		return nil, err
	}

	var descs []*desc.MethodDescriptor
	for _, svc := range allServices {
		if svc == "grpc.reflection.v1alpha.ServerReflection" {
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
		if cfg == nil && len(configs) != 0 {
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

func logErrorf(format string, args ...interface{}) {
	prefix := "ERROR: " + time.Now().Format("2006/01/02 15:04:05") + " "
	log.Printf(prefix+format, args...)
}

func logInfof(format string, args ...interface{}) {
	prefix := "INFO: " + time.Now().Format("2006/01/02 15:04:05") + " "
	log.Printf(prefix+format, args...)
}
