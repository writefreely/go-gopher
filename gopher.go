// Package gopher provides an implementation of the Gopher protocol (RFC 1436)
//
// Much of the API is similar in design to the net/http package of the
// standard library. To build custom Gopher servers implement handler
// functions or the `Handler{}` interface. Implementing a client is as
// simple as calling `gopher.Get(uri)` and passing in a `uri` such as
// `"gopher://gopher.floodgap.com/"`.
package gopher

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/net/context"
)

// Item Types
const (
	FILE        = ItemType('0') // Item is a file
	DIRECTORY   = ItemType('1') // Item is a directory
	PHONEBOOK   = ItemType('2') // Item is a CSO phone-book server
	ERROR       = ItemType('3') // Error
	BINHEX      = ItemType('4') // Item is a BinHexed Macintosh file.
	DOSARCHIVE  = ItemType('5') // Item is DOS binary archive of some sort. (*)
	UUENCODED   = ItemType('6') // Item is a UNIX uuencoded file.
	INDEXSEARCH = ItemType('7') // Item is an Index-Search server.
	TELNET      = ItemType('8') // Item points to a text-based telnet session.
	BINARY      = ItemType('9') // Item is a binary file! (*)

	// (*) Client must read until the TCP connection is closed.

	REDUNDANT = ItemType('+') // Item is a redundant server
	TN3270    = ItemType('T') // Item points to a text-based tn3270 session.
	GIF       = ItemType('g') // Item is a GIF format graphics file.
	IMAGE     = ItemType('I') // Item is some kind of image file.

	// non-standard
	INFO  = ItemType('i') // Item is an informational message
	HTML  = ItemType('h') // Item is a HTML document
	AUDIO = ItemType('s') // Item is an Audio file
	PNG   = ItemType('p') // Item is a PNG Image
	DOC   = ItemType('d') // Item is a Document
)

const (
	// END represents the terminator used in directory responses
	END = byte('.')

	// TAB is the delimiter used to separate item response parts
	TAB = byte('\t')

	// CRLF is the delimiter used per line of response item
	CRLF = "\r\n"

	// DEFAULT is the default item type
	DEFAULT = BINARY
)

// contextKey is a value for use with context.WithValue. It's used as
// a pointer so it fits in an interface{} without allocation.
type contextKey struct {
	name string
}

func (k *contextKey) String() string {
	return "gopher context value " + k.name
}

var (
	// ServerContextKey is a context key. It can be used in Gopher
	// handlers with context.WithValue to access the server that
	// started the handler. The associated value will be of type *Server.
	ServerContextKey = &contextKey{"gopher-server"}

	// LocalAddrContextKey is a context key. It can be used in
	// Gopher handlers with context.WithValue to access the address
	// the local address the connection arrived on.
	// The associated value will be of type net.Addr.
	LocalAddrContextKey = &contextKey{"local-addr"}
)

// ItemType represents the type of an item
type ItemType byte

// Return a human friendly represation of an ItemType
func (it ItemType) String() string {
	switch it {
	case FILE:
		return "TXT"
	case DIRECTORY:
		return "DIR"
	case PHONEBOOK:
		return "PHO"
	case ERROR:
		return "ERR"
	case BINHEX:
		return "HEX"
	case DOSARCHIVE:
		return "ARC"
	case UUENCODED:
		return "UUE"
	case INDEXSEARCH:
		return "QRY"
	case TELNET:
		return "TEL"
	case BINARY:
		return "BIN"
	case REDUNDANT:
		return "DUP"
	case TN3270:
		return "TN3"
	case GIF:
		return "GIF"
	case IMAGE:
		return "IMG"
	case INFO:
		return "NFO"
	case HTML:
		return "HTM"
	case AUDIO:
		return "SND"
	case PNG:
		return "PNG"
	case DOC:
		return "DOC"
	default:
		return "???"
	}
}

// Item describes an entry in a directory listing.
type Item struct {
	Type        ItemType `json:"type"`
	Description string   `json:"description"`
	Selector    string   `json:"selector"`
	Host        string   `json:"host"`
	Port        int      `json:"port"`

	// non-standard extensions (ignored by standard clients)
	Extras []string `json:"extras"`
}

// ParseItem parses a line of text into an item
func ParseItem(line string) (item *Item, err error) {
	parts := strings.Split(strings.Trim(line, "\r\n"), "\t")

	if len(parts[0]) < 1 {
		return nil, errors.New("no item type: " + string(line))
	}

	item = &Item{
		Type:        ItemType(parts[0][0]),
		Description: string(parts[0][1:]),
		Extras:      make([]string, 0),
	}

	// Selector
	if len(parts) > 1 {
		item.Selector = string(parts[1])
	} else {
		item.Selector = ""
	}

	// Host
	if len(parts) > 2 {
		item.Host = string(parts[2])
	} else {
		item.Host = "null.host"
	}

	// Port
	if len(parts) > 3 {
		port, err := strconv.Atoi(string(parts[3]))
		if err != nil {
			// Ignore parsing errors for bad servers for INFO types
			if item.Type != INFO {
				return nil, err
			}
			item.Port = 0
		}
		item.Port = port
	} else {
		item.Port = 0
	}

	// Extras
	if len(parts) >= 4 {
		for _, v := range parts[4:] {
			item.Extras = append(item.Extras, string(v))
		}
	}

	return
}

// MarshalJSON serializes an Item into a JSON structure
func (i *Item) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type        string   `json:"type"`
		Description string   `json:"description"`
		Selector    string   `json:"selector"`
		Host        string   `json:"host"`
		Port        int      `json:"port"`
		Extras      []string `json:"extras"`
	}{
		Type:        string(i.Type),
		Description: i.Description,
		Selector:    i.Selector,
		Host:        i.Host,
		Port:        i.Port,
		Extras:      i.Extras,
	})
}

// MarshalText serializes an Item into an array of bytes
func (i *Item) MarshalText() ([]byte, error) {
	b := []byte{}
	b = append(b, byte(i.Type))
	b = append(b, []byte(i.Description)...)
	b = append(b, TAB)
	b = append(b, []byte(i.Selector)...)
	b = append(b, TAB)
	b = append(b, []byte(i.Host)...)
	b = append(b, TAB)
	b = append(b, []byte(strconv.Itoa(i.Port))...)

	for _, s := range i.Extras {
		b = append(b, TAB)
		b = append(b, []byte(s)...)
	}

	b = append(b, []byte(CRLF)...)

	return b, nil
}

func (i *Item) isDirectoryLike() bool {
	switch i.Type {
	case DIRECTORY:
		return true
	case INDEXSEARCH:
		return true
	default:
		return false
	}
}

// Directory representes a Gopher Menu of Items
type Directory struct {
	Items []*Item `json:"items"`
}

// ToJSON returns the Directory as JSON bytes
func (d *Directory) ToJSON() ([]byte, error) {
	jsonBytes, err := json.Marshal(d)
	return jsonBytes, err
}

// ToText returns the Directory as UTF-8 encoded bytes
func (d *Directory) ToText() ([]byte, error) {
	var buffer bytes.Buffer
	for _, i := range d.Items {
		val, err := i.MarshalText()
		if err != nil {
			return nil, err
		}
		buffer.Write(val)
	}
	return buffer.Bytes(), nil
}

// Response represents a Gopher resource that
// Items contains a non-empty array of Item(s)
// for directory types, otherwise the Body
// contains the fetched resource (file, image, etc).
type Response struct {
	Type ItemType
	Dir  Directory
	Body io.Reader
}

// Get fetches a Gopher resource by URI
func Get(uri string) (*Response, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}

	if u.Scheme != "gopher" {
		return nil, errors.New("invalid scheme for uri")
	}

	var (
		host string
		port int
	)

	hostport := strings.Split(u.Host, ":")
	if len(hostport) == 2 {
		host = hostport[0]
		n, err := strconv.ParseInt(hostport[1], 10, 32)
		if err != nil {
			return nil, err
		}
		port = int(n)
	} else {
		host, port = hostport[0], 70
	}

	var (
		Type     ItemType
		Selector string
	)

	path := strings.TrimPrefix(u.Path, "/")
	if len(path) > 2 {
		Type = ItemType(path[0])
		Selector = path[1:]
		if u.RawQuery != "" {
			Selector += "\t" + u.RawQuery
		}
	} else if len(path) == 1 {
		Type = ItemType(path[0])
		Selector = ""
	} else {
		Type = ItemType(DIRECTORY)
		Selector = ""
	}

	i := Item{Type: Type, Selector: Selector, Host: host, Port: port}
	res := Response{Type: i.Type}

	if i.isDirectoryLike() {
		d, err := i.FetchDirectory()
		if err != nil {
			return nil, err
		}

		res.Dir = d
	} else {
		reader, err := i.FetchFile()
		if err != nil {
			return nil, err
		}

		res.Body = reader
	}

	return &res, nil
}

// FetchFile fetches data, not directory information.
// Calling this on a DIRECTORY Item type
// or unsupported type will return an error.
func (i *Item) FetchFile() (io.Reader, error) {
	if i.Type == DIRECTORY {
		return nil, errors.New("cannot fetch a directory as a file")
	}

	conn, err := net.Dial("tcp", i.Host+":"+strconv.Itoa(i.Port))
	if err != nil {
		return nil, err
	}

	_, err = conn.Write([]byte(i.Selector + CRLF))
	if err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}

// FetchDirectory fetches directory information, not data.
// Calling this on an Item whose type is not DIRECTORY will return an error.
func (i *Item) FetchDirectory() (Directory, error) {
	if !i.isDirectoryLike() {
		return Directory{}, errors.New("cannot fetch a file as a directory")
	}

	conn, err := net.Dial("tcp", i.Host+":"+strconv.Itoa(i.Port))
	if err != nil {
		return Directory{}, err
	}

	_, err = conn.Write([]byte(i.Selector + CRLF))
	if err != nil {
		return Directory{}, err
	}

	reader := bufio.NewReader(conn)
	scanner := bufio.NewScanner(reader)
	scanner.Split(bufio.ScanLines)

	var items []*Item

	for scanner.Scan() {
		line := strings.Trim(scanner.Text(), "\r\n")

		if len(line) == 0 {
			continue
		}

		if len(line) == 1 && line[0] == END {
			break
		}

		item, err := ParseItem(line)
		if err != nil {
			log.Printf("Error parsing %q: %q", line, err)
			continue
		}
		items = append(items, item)
	}

	return Directory{items}, nil
}

// Request repsesnts an inbound request to a listening server.
// LocalHost and LocalPort may be used by the Handler for local links.
// These are specified in the call to ListenAndServe.
type Request struct {
	conn      net.Conn
	Selector  string
	LocalHost string
	LocalPort int
}

// A Handler responds to a Gopher request.
//
// ServeGopher should write data or items to the ResponseWriter
// and then return. Returning signals that the request is finished; it
// is not valid to use the ResponseWriter concurrently with the completion
// of the ServeGopher call.
//
// Handlers should not modify the provided request.
//
// If ServeGopher panics, the server (the caller of ServeGopher) assumes
// that the effect of the panic was isolated to the active request.
// It recovers the panic, logs a stack trace to the server error log,
// and hangs up the connection.
type Handler interface {
	ServeGopher(ResponseWriter, *Request)
}

// FileExtensions defines a mapping of known file extensions to gopher types
var FileExtensions = map[string]ItemType{
	".txt":  FILE,
	".gif":  GIF,
	".jpg":  IMAGE,
	".jpeg": IMAGE,
	".png":  IMAGE,
	".html": HTML,
	".ogg":  AUDIO,
	".mp3":  AUDIO,
	".wav":  AUDIO,
	".mod":  AUDIO,
	".it":   AUDIO,
	".xm":   AUDIO,
	".mid":  AUDIO,
	".vgm":  AUDIO,
	".s":    FILE,
	".c":    FILE,
	".py":   FILE,
	".h":    FILE,
	".md":   FILE,
	".go":   FILE,
	".fs":   FILE,
}

// MimeTypes defines a mapping of known mimetypes to gopher types
var MimeTypes = map[string]ItemType{
	"text/html": HTML,
	"text/*":    FILE,

	"image/gif": GIF,
	"image/*":   IMAGE,

	"audio/*": AUDIO,

	"application/x-tar":  DOSARCHIVE,
	"application/x-gtar": DOSARCHIVE,

	"application/x-xz":    DOSARCHIVE,
	"application/x-zip":   DOSARCHIVE,
	"application/x-gzip":  DOSARCHIVE,
	"application/x-bzip2": DOSARCHIVE,
}

func matchExtension(f os.FileInfo) ItemType {
	extension := strings.ToLower(filepath.Ext(f.Name()))
	k, ok := FileExtensions[extension]
	if !ok {
		return DEFAULT
	}
	return k
}

func matchMimeType(mimeType string) ItemType {
	for k, v := range MimeTypes {
		matched, err := filepath.Match(k, mimeType)
		if !matched || (err != nil) {
			continue
		}
		return v
	}
	return DEFAULT
}

// GetItemType returns the Gopher Type of the given path
func GetItemType(p string) ItemType {
	fi, err := os.Stat(p)
	if err != nil {
		return DEFAULT
	}

	if fi.IsDir() {
		return DIRECTORY
	}

	f, err := os.Open(p)
	if err != nil {
		return matchExtension(fi)
	}

	b := make([]byte, 512)
	n, err := io.ReadAtLeast(f, b, 512)
	if (err != nil) || (n != 512) {
		return matchExtension(fi)
	}

	mimeType := http.DetectContentType(b)
	mimeParts := strings.Split(mimeType, ";")
	return matchMimeType(mimeParts[0])
}

// Server defines parameters for running a Gopher server.
// A zero value for Server is valid configuration.
type Server struct {
	Addr    string  // TCP address to listen on, ":gopher" if empty
	Handler Handler // handler to invoke, gopher.DefaultServeMux if nil

	Hostname string // FQDN Hostname to reach this server on

	// ErrorLog specifies an optional logger for errors accepting
	// connections and unexpected behavior from handlers.
	// If nil, logging goes to os.Stderr via the log package's
	// standard logger.
	ErrorLog *log.Logger
}

// serverHandler delegates to either the server's Handler or
// DefaultServeMux and also handles "OPTIONS *" requests.
type serverHandler struct {
	s *Server
}

func (sh serverHandler) ServeGopher(rw ResponseWriter, req *Request) {
	handler := sh.s.Handler
	if handler == nil {
		handler = DefaultServeMux
	}
	handler.ServeGopher(rw, req)
}

// ListenAndServe starts serving gopher requests using the given Handler.
// The address passed to ListenAndServe should be an internet-accessable
// domain name, optionally followed by a colon and the port number.
//
// If the address is not a FQDN, LocalHost as passed to the Handler
// may not be accessible to clients, so links may not work.
func (s *Server) ListenAndServe() error {
	addr := s.Addr
	if addr == "" {
		addr = ":70"
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	return s.Serve(ln)
}

// ListenAndServeTLS listens on the TCP network address srv.Addr and
// then calls Serve to handle requests on incoming TLS connections.
// Accepted connections are configured to enable TCP keep-alives.
//
// Filenames containing a certificate and matching private key for the
// server must be provided if neither the Server's TLSConfig.Certificates
// nor TLSConfig.GetCertificate are populated. If the certificate is
// signed by a certificate authority, the certFile should be the
// concatenation of the server's certificate, any intermediates, and
// the CA's certificate.
//
// If srv.Addr is blank, ":gophers" is used (port 73).
//
// ListenAndServeTLS always returns a non-nil error.
func (s *Server) ListenAndServeTLS(certFile, keyFile string) error {
	addr := s.Addr
	if addr == "" {
		addr = ":73"
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		log.Fatalf("server: loadkeys: %s", err)
	}
	config := tls.Config{Certificates: []tls.Certificate{cert}}
	config.Rand = rand.Reader

	ln, err := tls.Listen("tcp", addr, &config)
	if err != nil {
		log.Fatalf("server: listen: %s", err)
	}

	return s.Serve(ln)
}

// Serve ...
func (s *Server) Serve(l net.Listener) error {
	defer l.Close()

	ctx := context.Background()
	ctx = context.WithValue(ctx, ServerContextKey, s)
	ctx = context.WithValue(ctx, LocalAddrContextKey, l.Addr())

	for {
		rw, err := l.Accept()
		if err != nil {
			fmt.Errorf("error acceptig new client: %s", err)
			return err
		}

		c := s.newConn(rw)
		go c.serve(ctx)
	}
}

// A conn represents the server side of a Gopher connection.
type conn struct {
	// server is the server on which the connection arrived.
	// Immutable; never nil.
	server *Server

	// rwc is the underlying network connection.
	// This is never wrapped by other types and is the value given out
	// to CloseNotifier callers. It is usually of type *net.TCPConn or
	// *tls.Conn.
	rwc net.Conn

	// remoteAddr is rwc.RemoteAddr().String(). It is not populated synchronously
	// inside the Listener's Accept goroutine, as some implementations block.
	// It is populated immediately inside the (*conn).serve goroutine.
	// This is the value of a Handler's (*Request).RemoteAddr.
	remoteAddr string

	// tlsState is the TLS connection state when using TLS.
	// nil means not TLS.
	tlsState *tls.ConnectionState

	// mu guards hijackedv, use of bufr, (*response).closeNotifyCh.
	mu sync.Mutex
}

// Create new connection from rwc.
func (s *Server) newConn(rwc net.Conn) *conn {
	c := &conn{
		server: s,
		rwc:    rwc,
	}
	return c
}

func (c *conn) serve(ctx context.Context) {
	c.remoteAddr = c.rwc.RemoteAddr().String()

	w, err := c.readRequest(ctx)

	if err != nil {
		if err == io.EOF {
			return // don't reply
		}
		if neterr, ok := err.(net.Error); ok && neterr.Timeout() {
			return // don't reply
		}
		io.WriteString(c.rwc, "3\tbad request\terror.host\t0")
		return
	}

	serverHandler{c.server}.ServeGopher(w, w.req)
	w.End()
}

func readRequest(rwc net.Conn) (req *Request, err error) {
	reader := bufio.NewReader(rwc)
	scanner := bufio.NewScanner(reader)
	scanner.Split(bufio.ScanLines)
	scanner.Scan()

	req = &Request{
		Selector: scanner.Text(),
	}

	// If empty selector, assume /
	if req.Selector == "" {
		req.Selector = "/"
	}

	// If no leading / prefix, add one
	if !strings.HasPrefix(req.Selector, "/") {
		req.Selector = "/" + req.Selector
	}

	return req, nil
}

func (c *conn) close() (err error) {
	c.mu.Lock() // while using bufr
	err = c.rwc.Close()
	c.mu.Unlock()
	return
}

func (c *conn) readRequest(ctx context.Context) (w *response, err error) {
	c.mu.Lock() // while using bufr
	req, err := readRequest(c.rwc)
	c.mu.Unlock()
	if err != nil {
		return nil, err
	}

	localaddr := ctx.Value(LocalAddrContextKey).(*net.TCPAddr)
	host, port, err := net.SplitHostPort(localaddr.String())
	if err != nil {
		return nil, err
	}

	n, err := strconv.ParseInt(port, 10, 32)
	if err != nil {
		return nil, err
	}

	server := ctx.Value(ServerContextKey).(*Server)
	if server.Hostname == "" {
		req.LocalHost = host
		req.LocalPort = int(n)
	} else {
		req.LocalHost = server.Hostname
		// TODO: Parse this from -bind option
		req.LocalPort = int(n)
	}

	w = &response{
		conn: c,
		req:  req,
	}
	w.w = bufio.NewWriter(c.rwc)

	return w, nil
}

func (s *Server) logf(format string, args ...interface{}) {
	if s.ErrorLog != nil {
		s.ErrorLog.Printf(format, args...)
	} else {
		log.Printf(format, args...)
	}
}

// ListenAndServe listens on the TCP network address addr
// and then calls Serve with handler to handle requests
// on incoming connections.
//
// A trivial example server is:
//
//    package main
//
//    import (
//        "io"
//        "log"
//
//        "github.com/prologic/go-gopher"
//    )
//
//    // hello world, the gopher server
//    func HelloServer(w gopher.ResponseWriter, req *gopher.Request) {
//        w.WriteInfo("hello, world!")
//    }
//
//    func main() {
//        gopher.HandleFunc("/hello", HelloServer)
//        log.Fatal(gopher.ListenAndServe(":7000", nil))
//    }
//
// ListenAndServe always returns a non-nil error.
func ListenAndServe(addr string, handler Handler) error {
	server := &Server{Addr: addr, Handler: handler}
	return server.ListenAndServe()
}

// ListenAndServeTLS acts identically to ListenAndServe, except that it
// expects TLS connections. Additionally, files containing a certificate and
// matching private key for the server must be provided. If the certificate
// is signed by a certificate authority, the certFile should be the
// concatenation of the server's certificate, any intermediates,
// and the CA's certificate.
//
// A trivial example server is:
//
//    import (
//        "log"
//
//        "github.com/prologic/go-gopher",
//    )
//
//    func HelloServer(w gopher.ResponseWriter, req *gopher.Request) {
//        w.WriteInfo("hello, world!")
//    }
//
//    func main() {
//        gopher.HandleFunc("/", handler)
//        log.Printf("About to listen on 73. Go to gophers://127.0.0.1:73/")
//        err := gopher.ListenAndServeTLS(":73", "cert.pem", "key.pem", nil)
//        log.Fatal(err)
//    }
//
// One can use generate_cert.go in crypto/tls to generate cert.pem and key.pem.
//
// ListenAndServeTLS always returns a non-nil error.
func ListenAndServeTLS(addr, certFile, keyFile string, handler Handler) error {
	server := &Server{Addr: addr, Handler: handler}
	return server.ListenAndServeTLS(certFile, keyFile)
}

// ServeMux is a Gopher request multiplexer.
// It matches the URL of each incoming request against a list of registered
// patterns and calls the handler for the pattern that
// most closely matches the URL.
//
// Patterns name fixed, rooted paths, like "/favicon.ico",
// or rooted subtrees, like "/images/" (note the trailing slash).
// Longer patterns take precedence over shorter ones, so that
// if there are handlers registered for both "/images/"
// and "/images/thumbnails/", the latter handler will be
// called for paths beginning "/images/thumbnails/" and the
// former will receive requests for any other paths in the
// "/images/" subtree.
//
// Note that since a pattern ending in a slash names a rooted subtree,
// the pattern "/" matches all paths not matched by other registered
// patterns, not just the URL with Path == "/".
//
// If a subtree has been registered and a request is received naming the
// subtree root without its trailing slash, ServeMux redirects that
// request to the subtree root (adding the trailing slash). This behavior can
// be overridden with a separate registration for the path without
// the trailing slash. For example, registering "/images/" causes ServeMux
// to redirect a request for "/images" to "/images/", unless "/images" has
// been registered separately.
//
// ServeMux also takes care of sanitizing the URL request path,
// redirecting any request containing . or .. elements or repeated slashes
// to an equivalent, cleaner URL.
type ServeMux struct {
	mu sync.RWMutex
	m  map[string]muxEntry
}

type muxEntry struct {
	explicit bool
	h        Handler
	pattern  string
}

// NewServeMux allocates and returns a new ServeMux.
func NewServeMux() *ServeMux { return new(ServeMux) }

// DefaultServeMux is the default ServeMux used by Serve.
var DefaultServeMux = &defaultServeMux

var defaultServeMux ServeMux

// Does selector match pattern?
func selectorMatch(pattern, selector string) bool {
	if len(pattern) == 0 {
		// should not happen
		return false
	}
	n := len(pattern)
	if pattern[n-1] != '/' {
		return pattern == selector
	}
	return len(selector) >= n && selector[0:n] == pattern
}

// Return the canonical path for p, eliminating . and .. elements.
func cleanPath(p string) string {
	if p == "" {
		return "/"
	}
	if p[0] != '/' {
		p = "/" + p
	}
	np := path.Clean(p)
	// path.Clean removes trailing slash except for root;
	// put the trailing slash back if necessary.
	if p[len(p)-1] == '/' && np != "/" {
		np += "/"
	}
	return np
}

// Find a handler on a handler map given a path string
// Most-specific (longest) pattern wins
func (mux *ServeMux) match(selector string) (h Handler, pattern string) {
	var n = 0
	for k, v := range mux.m {
		if !selectorMatch(k, selector) {
			continue
		}
		if h == nil || len(k) > n {
			n = len(k)
			h = v.h
			pattern = v.pattern
		}
	}
	return
}

// Handler returns the handler to use for the given request,
// consulting r.Selector. It always returns
// a non-nil handler.
//
// Handler also returns the registered pattern that matches the request.
//
// If there is no registered handler that applies to the request,
// Handler returns a ``resource not found'' handler and an empty pattern.
func (mux *ServeMux) Handler(r *Request) (h Handler, pattern string) {
	return mux.handler(r.Selector)
}

// handler is the main implementation of Handler.
func (mux *ServeMux) handler(selector string) (h Handler, pattern string) {
	mux.mu.RLock()
	defer mux.mu.RUnlock()

	h, pattern = mux.match(selector)
	if h == nil {
		h, pattern = NotFoundHandler(), ""
	}

	return
}

// ServeGopher dispatches the request to the handler whose
// pattern most closely matches the request URL.
func (mux *ServeMux) ServeGopher(w ResponseWriter, r *Request) {
	h, _ := mux.Handler(r)
	h.ServeGopher(w, r)
}

// Handle registers the handler for the given pattern.
// If a handler already exists for pattern, Handle panics.
func (mux *ServeMux) Handle(pattern string, handler Handler) {
	mux.mu.Lock()
	defer mux.mu.Unlock()

	if pattern == "" {
		panic("gopher: invalid pattern " + pattern)
	}
	if handler == nil {
		panic("gopher: nil handler")
	}
	if mux.m[pattern].explicit {
		panic("gopher: multiple registrations for " + pattern)
	}

	if mux.m == nil {
		mux.m = make(map[string]muxEntry)
	}
	mux.m[pattern] = muxEntry{explicit: true, h: handler, pattern: pattern}
}

// HandleFunc registers the handler function for the given pattern.
func (mux *ServeMux) HandleFunc(pattern string, handler func(ResponseWriter, *Request)) {
	mux.Handle(pattern, HandlerFunc(handler))
}

// The HandlerFunc type is an adapter to allow the use of
// ordinary functions as Gopher handlers. If f is a function
// with the appropriate signature, HandlerFunc(f) is a
// Handler that calls f.
type HandlerFunc func(ResponseWriter, *Request)

// ServeGopher calls f(w, r).
func (f HandlerFunc) ServeGopher(w ResponseWriter, r *Request) {
	f(w, r)
}

// A ResponseWriter interface is used by a Gopher handler to
// construct an Gopher response.
//
// A ResponseWriter may not be used after the Handler.ServeGopher method
// has returned.
type ResponseWriter interface {
	// Server returns the connection's server instance
	Server() *Server

	// End ends the document by writing the terminating period and crlf
	End() error

	// Write writes the data to the connection as part of a Gopher reply.
	//
	Write([]byte) (int, error)

	// WriteError writes an error item
	WriteError(err string) error

	// WriteInfo writes an informational item
	WriteInfo(msg string) error

	// WriteItem writes an item
	WriteItem(i *Item) error
}

// A response represents the server side of a Gopher response.
type response struct {
	conn *conn

	req *Request // request for this response

	w *bufio.Writer // buffers output

	rt int
}

func (w *response) Server() *Server {
	return w.conn.server
}

func (w *response) Write(b []byte) (int, error) {
	if w.rt == 0 {
		w.rt = 1
	}

	if w.rt != 1 {
		return 0, errors.New("cannot write document data to a directory")
	}

	return w.w.Write(b)
}

func (w *response) WriteError(err string) error {
	if w.rt == 0 {
		w.rt = 2
	}

	if w.rt != 2 {
		_, e := w.w.Write([]byte(err))
		return e
	}

	i := &Item{
		Type:        ERROR,
		Description: err,
		Host:        "error.host",
		Port:        1,
	}

	return w.WriteItem(i)
}

func (w *response) WriteInfo(msg string) error {
	if w.rt == 0 {
		w.rt = 2
	}

	if w.rt != 2 {
		_, e := w.w.Write([]byte(msg))
		return e
	}

	i := &Item{
		Type:        INFO,
		Description: msg,
		Host:        "error.host",
		Port:        1,
	}

	return w.WriteItem(i)
}

func (w *response) WriteItem(i *Item) error {
	if w.rt == 0 {
		w.rt = 2
	}

	if w.rt != 2 {
		return errors.New("cannot write directory data to a document")
	}

	if i.Host == "" && i.Port == 0 {
		i.Host = w.req.LocalHost
		i.Port = w.req.LocalPort
	}

	b, err := i.MarshalText()
	if err != nil {
		return err
	}

	_, err = w.w.Write(b)
	if err != nil {
		return err
	}

	return nil
}

func (w *response) End() (err error) {
	if w.rt == 2 {
		_, err = w.w.Write(append([]byte{END}, CRLF...))
		if err != nil {
			return
		}
	}

	err = w.w.Flush()
	if err != nil {
		return
	}

	err = w.conn.close()
	if err != nil {
		return
	}

	return
}

// Helper handlers

// Error replies to the request with the specified error message.
// It does not otherwise end the request; the caller should ensure no further
// writes are done to w.
// The error message should be plain text.
func Error(w ResponseWriter, error string) {
	w.WriteError(error)
}

// NotFound replies to the request with an resouce not found error item.
func NotFound(w ResponseWriter, r *Request) {
	Error(w, "resource not found")
}

// NotFoundHandler returns a simple request handler
// that replies to each request with a ``resource page not found'' reply.
func NotFoundHandler() Handler { return HandlerFunc(NotFound) }

type fileHandler struct {
	root FileSystem
}

// FileServer returns a handler that serves Gopher requests
// with the contents of the file system rooted at root.
//
// To use the operating system's file system implementation,
// use gopher.Dir:
//
//     gopher.Handle("/", gopher.FileServer(gopher.Dir("/tmp")))
func FileServer(root FileSystem) Handler {
	return &fileHandler{root}
}

func (f *fileHandler) ServeGopher(w ResponseWriter, r *Request) {
	upath := r.Selector
	if !strings.HasPrefix(upath, "/") {
		upath = "/" + upath
		r.Selector = upath
	}
	serveFile(w, r, f.root, path.Clean(upath))
}

// A Dir implements FileSystem using the native file system restricted to a
// specific directory tree.
//
// While the FileSystem.Open method takes '/'-separated paths, a Dir's string
// value is a filename on the native file system, not a URL, so it is separated
// by filepath.Separator, which isn't necessarily '/'.
//
// An empty Dir is treated as ".".
type Dir string

// Name returns the directory
func (d Dir) Name() string {
	return string(d)
}

// Open opens the directory
func (d Dir) Open(name string) (File, error) {
	if filepath.Separator != '/' &&
		strings.ContainsRune(name, filepath.Separator) ||
		strings.Contains(name, "\x00") {
		return nil, errors.New("gopher: invalid character in file path")
	}
	dir := string(d)
	if dir == "" {
		dir = "."
	}
	f, err := os.Open(
		filepath.Join(dir, filepath.FromSlash(path.Clean("/"+name))),
	)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// A FileSystem implements access to a collection of named files.
// The elements in a file path are separated by slash ('/', U+002F)
// characters, regardless of host operating system convention.
type FileSystem interface {
	Name() string
	Open(name string) (File, error)
}

// A File is returned by a FileSystem's Open method and can be
// served by the FileServer implementation.
//
// The methods should behave the same as those on an *os.File.
type File interface {
	io.Closer
	io.Reader
	io.Seeker
	Readdir(count int) ([]os.FileInfo, error)
	Stat() (os.FileInfo, error)
}

func dirList(w ResponseWriter, r *Request, f File, fs FileSystem) {
	root := fs.Name()

	fullpath := f.(*os.File).Name()

	files, err := f.Readdir(-1)
	if err != nil {
		// TODO: log err.Error() to the Server.ErrorLog, once it's possible
		// for a handler to get at its Server via the ResponseWriter.
		Error(w, "Error reading directory")
		return
	}
	sort.Sort(byName(files))

	for _, file := range files {
		if file.Name()[0] == '.' {
			continue
		}
		if file.Mode()&os.ModeDir != 0 {
			pathname, err := filepath.Rel(
				root,
				path.Join(fullpath, file.Name()),
			)
			if err != nil {
				Error(w, "Error reading directory")
				return
			}
			w.WriteItem(&Item{
				Type:        DIRECTORY,
				Description: file.Name(),
				Selector:    pathname,
				Host:        r.LocalHost,
				Port:        r.LocalPort,
			})
		} else if file.Mode()&os.ModeType == 0 {
			pathname, err := filepath.Rel(
				root,
				path.Join(fullpath, file.Name()),
			)
			if err != nil {
				Error(w, "Error reading directory")
				return
			}

			itemtype := GetItemType(path.Join(fullpath, file.Name()))

			w.WriteItem(&Item{
				Type:        itemtype,
				Description: file.Name(),
				Selector:    pathname,
				Host:        r.LocalHost,
				Port:        r.LocalPort,
			})
		}
	}
}

type byName []os.FileInfo

func (s byName) Len() int           { return len(s) }
func (s byName) Less(i, j int) bool { return s[i].Name() < s[j].Name() }
func (s byName) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// name is '/'-separated, not filepath.Separator.
func serveFile(w ResponseWriter, r *Request, fs FileSystem, name string) {
	const gophermapFile = "/gophermap"

	f, err := fs.Open(name)
	if err != nil {
		Error(w, err.Error())
		return
	}
	defer f.Close()

	d, err := f.Stat()
	if err != nil {
		Error(w, err.Error())
		return
	}

	// use contents of gophermap for directory, if present
	if d.IsDir() {
		gophermap := strings.TrimSuffix(name, "/") + gophermapFile
		ff, err := fs.Open(gophermap)
		if err == nil {
			defer ff.Close()
			dd, err := ff.Stat()
			if err == nil {
				name = gophermap
				d = dd
				f = ff
			}
		}
	}

	// Still a directory? (we didn't find a gophermap file)
	if d.IsDir() {
		dirList(w, r, f, fs)
		return
	}

	serveContent(w, r, f)
}

// content must be seeked to the beginning of the file.
func serveContent(w ResponseWriter, r *Request, content io.ReadSeeker) {
	io.Copy(w, content)
}

// Handle registers the handler for the given pattern
// in the DefaultServeMux.
// The documentation for ServeMux explains how patterns are matched.
func Handle(pattern string, handler Handler) {
	DefaultServeMux.Handle(pattern, handler)
}

// HandleFunc registers the handler function for the given pattern
// in the DefaultServeMux.
// The documentation for ServeMux explains how patterns are matched.
func HandleFunc(pattern string, handler func(ResponseWriter, *Request)) {
	DefaultServeMux.HandleFunc(pattern, handler)
}
