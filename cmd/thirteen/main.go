// The Thirteen Gopher server
// Copyright 2025 Christopher Williams
// SPDX-License-Identifier: GPL-2.0-only
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

const (
	serverSoftwareName    = "Thirteen"
	serverSoftwareVersion = "0.0.0"
	serverSoftware        = serverSoftwareName + "/" + serverSoftwareVersion
)

var (
	// must read a request within this time
	requestReadTimeout time.Duration

	// must write at least one byte to the client during this time
	responseProgressTimeout time.Duration

	docRoot string

	excluded = make(map[string]bool, 10)

	startTime = time.Now()
)

type configOption struct {
	usage string
	value interface{}
}

var configMap = map[string]configOption{
	"desc": configOption{
		"The server `description`.",
		newString(""),
	},
	"listen": configOption{
		"The `[host:]port` to listen on.",
		newString("70"),
	},
	"maxconn": configOption{
		"The maximum number of simultaneous `connections`.",
		newInt(1000),
	},
	"root": configOption{
		"The site root `directory`.",
		newString(defaultSiteRoot),
	},
	"rtmo": configOption{
		"Request timeout in `seconds`. How long to wait to\n" +
			"receive a complete request. Setting to 0\n" +
			"disables request timeout (not recommended).",
		newInt(60),
	},
	"serverhost": configOption{
		"The server host `name`.",
		newString("localhost"),
	},
	"serverport": configOption{
		"The `port` to include in menus.",
		newInt(0),
	},
	"user": configOption{
		"The `user` to run as.",
		newString(""),
	},
	"wtmo": configOption{
		"Response timeout in `seconds`. How long to wait\n" +
			"for progress to be made on the response. Setting\n" +
			"to 0 disables response timeout.",
		newInt(300),
	},
}

func newString(v string) *string { p := new(string); *p = v; return p }
func newInt(v int) *int          { p := new(int); *p = v; return p }

func configInt(name string) int {
	if u, ok := configMap[name].value.(*int); ok {
		return *u
	}
	return 0
}
func configString(name string) string {
	switch u := configMap[name].value.(type) {
	case *int:
		return fmt.Sprintf("%d", *u)
	case *string:
		return *u
	default:
		return "unsupport type"
	}
}
func setConfigInt(name string, value int) {
	if u, ok := configMap[name].value.(*int); ok {
		*u = value
	}
}
func setConfigString(name string, value string) {
	if u, ok := configMap[name].value.(*string); ok {
		*u = value
	}
}

var connChan chan struct{}

func main() {
	// get config from arguments
	for name, option := range configMap {
		switch u := option.value.(type) {
		case *int:
			flag.IntVar(u, string(name), *u, option.usage)
		case *string:
			flag.StringVar(u, string(name), *u, option.usage)
		default:
			panic("unsupported type")
		}
	}
	flag.Func("exclude", "Exclude files with the given `extension`.", func(ext string) error {
		if ext == "" {
			// don't exclude blank
			return nil
		}
		if !strings.HasPrefix(ext, ".") {
			// extension is missing the leading dot? that's alright!
			ext = "." + ext
		}
		if strings.Contains(ext[1:], ".") {
			return fmt.Errorf("extension contains two or more dots")
		}
		excluded[ext] = true
		return nil
	})
	flag.Parse()

	maxConn := configInt("maxconn")
	if maxConn < 1 {
		fmt.Fprintln(os.Stderr, "Error: maxconn must be > 0.")
		return
	}

	r := configInt("rtmo")
	if r < 0 {
		fmt.Fprintln(os.Stderr, "Error: rtmo must be >= 0.")
		return
	}
	requestReadTimeout = time.Duration(r) * time.Second

	w := configInt("wtmo")
	if w < 0 {
		fmt.Fprintln(os.Stderr, "Error: wtmo must be >= 0.")
		return
	}
	responseProgressTimeout = time.Duration(w) * time.Second

	hostPortRe := regexp.MustCompilePOSIX(`^((.*):)?([^:]*)$`)

	listen := configString("listen")
	listenHost := hostPortRe.ReplaceAllString(listen, "$2")
	listenPort := hostPortRe.ReplaceAllString(listen, "$3")

	var err error
	var port int

	if port, err = strconv.Atoi(listenPort); err != nil || port <= 0 || 65535 < port {
		fmt.Fprintln(os.Stderr, "Error: port must be between 1 and 65535.")
		return
	}

	docRoot, err = filepath.Abs(configString("root"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	if serverport := configInt("serverport"); serverport == 0 {
		setConfigInt("serverport", port)
	}

	connChan = make(chan struct{}, maxConn)

	listener, err := net.Listen("tcp", listenHost+":"+listenPort)
	if err != nil {
		fmt.Print("net.Listen: " + err.Error())
		os.Exit(1)
	}

	if user := configString("user"); user != "" {
		err = changeUser(user)
		if err != nil {
			fmt.Print("changeUser: " + err.Error())
			os.Exit(1)
		}
	}

	for {
		connChan <- struct{}{}
		conn, err := listener.Accept()
		if err != nil {
			fmt.Print("Accept: " + err.Error())
			continue
		}
		go handleConn(conn)
	}
}

type statusCode int

const (
	okStatus                  statusCode = 200
	badRequestStatus          statusCode = 400
	forbiddenStatus           statusCode = 403
	fileNotFoundStatus        statusCode = 404
	internalServerErrorStatus statusCode = 500
)

type responseError struct {
	status  statusCode
	message string
}

var (
	badRequestError          = &responseError{badRequestStatus, "Bad request."}
	fileNotFoundError        = &responseError{fileNotFoundStatus, "File not found."}
	forbiddenError           = &responseError{forbiddenStatus, "Forbidden."}
	internalServerErrorError = &responseError{internalServerErrorStatus, "Internal server error."}
)

type response struct {
	io.Reader
	status statusCode
	cmd    *exec.Cmd // for CGIs
}

type requestInfo struct {
	host        string
	requestTime time.Time
	request     []byte
	status      statusCode
	transferred uint64
}

var (
	requestCount     atomic.Uint64
	bytesTransferred atomic.Uint64
)

func (r *requestInfo) log() {
	transferred := "-"
	if r.transferred != 0 {
		transferred = fmt.Sprintf("%d", r.transferred)
	}
	fmt.Fprintf(os.Stderr, "%s %s %s [%s] %q %d %s\n", r.host, "-", "-", r.requestTime.Format(time.RFC3339), r.request, r.status, transferred)

	requestCount.Add(1)
	bytesTransferred.Add(r.transferred)
}

func handleConn(conn net.Conn) {
	defer func() { _ = <-connChan }()
	defer conn.Close()

	var remoteAddr string
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		remoteAddr = tcpConn.RemoteAddr().String()
		remoteAddr, _, _ = strings.Cut(remoteAddr, ":")
	}

	var response response
	var requestInfo requestInfo

	request, err := readRequest(conn)
	if err != nil {
		response = makeErrorResponse(badRequestError)
	} else {
		selector, path, query, search := splitRequest(request)

		response = getResponseForRequest(conn, selector, path, query, search)
		if response.cmd != nil {
			defer response.cmd.Wait()
		}
	}

	requestInfo.requestTime = time.Now()
	requestInfo.request = request
	requestInfo.host = remoteAddr

	if closer, ok := response.Reader.(io.Closer); ok {
		defer closer.Close()
	}

	requestInfo.status = response.status

	defer requestInfo.log()

	for {
		buf := make([]byte, 1000)
		n, err := response.Read(buf)
		if n == 0 {
			// end of response
			break
		}

		if responseProgressTimeout != 0 {
			conn.SetWriteDeadline(time.Now().Add(responseProgressTimeout))
		}
		n, err = conn.Write(buf[:n])
		requestInfo.transferred += uint64(n)
		// if an error happened or nothing transfers then we're done
		if err != nil || n == 0 {
			break
		}
	}
}

// Read a request from the client.
//
// A request must end in either LF or CR LF (CR, if present, must be
// immediately before LF). A request also must not contain a NUL byte.
func readRequest(conn net.Conn) ([]byte, error) {
	const initialRequestSize = 256
	const maxRequestSize = 16384
	const readChunkSize = 4096

	requestBuf := make([]byte, 0, initialRequestSize)
	buf := make([]byte, readChunkSize)
	reader := bufio.NewReader(conn)
	if requestReadTimeout != 0 {
		conn.SetReadDeadline(time.Now().Add(requestReadTimeout))
	}
	for len(requestBuf) < maxRequestSize {
		n, err := reader.Read(buf)
		if err != nil {
			return nil, err
		}
		if bytes.Contains(buf[:n], []byte{0}) {
			return nil, fmt.Errorf("bad request")
		}

		before, _, foundLF := bytes.Cut(buf[:n], []byte{'\n'})

		requestBuf = append(requestBuf, before...)

		if foundLF {
			// see if there's a CR
			crIndex := bytes.IndexByte(requestBuf, '\r')
			if crIndex != -1 {
				// if the CR is not at the end, it's a bad request
				lastByte := len(requestBuf) - 1
				if crIndex != lastByte {
					return nil, fmt.Errorf("bad request")
				}
				requestBuf = requestBuf[:lastByte]
			}
			return requestBuf, nil
		}
	}
	return nil, fmt.Errorf("request too big")
}

// Split a request into selector, path, query, and search strings.
//
// Request is split (by a tab) into `selector` and `search`.
//
// The selector is further split (by a `?`) into `path` and `query`.
func splitRequest(request []byte) (selector, path, query, search string) {
	tabBytes := []byte{'\t'}
	questionMarkBytes := []byte{'?'}

	// Cut at the first tab.
	selectorBytes, searchBytes, _ := bytes.Cut(request, tabBytes)

	// Remove everything following a tab (if any--only for Gopher+ clients).
	// (If I read the Gopher+ spec correctly, it sends a request with only
	// *one* tab for non-search selectors, which can be confused easily
	// with a search string.)
	searchBytes, _, _ = bytes.Cut(searchBytes, tabBytes)

	var pathBytes, queryBytes []byte

	// Split selector into path and query string (if any).
	pathBytes, queryBytes, _ = bytes.Cut(selectorBytes, questionMarkBytes)

	selector, path, query, search = string(selectorBytes), string(pathBytes), string(queryBytes), string(searchBytes)

	return
}

const cgiExt = ".cgi"
const indexCgiPath = "/index.cgi"

var indexPaths = []string{
	indexCgiPath,
	"/index.map",
}

// Open the file or whatever and return a response.
func getResponseForRequest(conn net.Conn, selector, path, query, search string) response {
	fsPath, scriptName, pathInfo, err := splitPath(docRoot, path)
	if err != nil {
		return makeErrorResponse(err)
	}

	return getResponseFromPath(conn, selector, fsPath, scriptName, pathInfo, query, search)
}

func getResponseFromPath(conn net.Conn, selector, fsPath, scriptName, pathInfo, query, search string) response {
	f, err := os.Open(fsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return makeErrorResponse(fileNotFoundError)
		} else {
			return makeErrorResponse(forbiddenError)
		}
	}

	if strings.HasSuffix(fsPath, cgiExt) {
		f.Close()

		// TODO check that it's executable
		return runCGI(conn, selector, fsPath, scriptName, pathInfo, query, search)
	}

	// only CGIs can have extra path information
	if pathInfo != "" {
		f.Close()

		return makeErrorResponse(fileNotFoundError)
	}

	return response{f, okStatus, nil}
}

func runCGI(conn net.Conn, selector, fsPath, scriptName, pathInfo, query, search string) response {
	// pass info as command-line arguments as per geomyidae
	cmd := exec.Command(fsPath,
		// Query string (type 7) or "" (type 0).
		search,

		// String behind "?" in selector or "".
		query,

		// Server's hostname.
		configString("serverhost"),

		// Server's port.
		configString("serverport"),

		// Remaining path from path traversal in REST case.
		pathInfo,

		// Raw selector or full req.
		selector,
	)

	if lastSlash := strings.LastIndex(fsPath, "/"); lastSlash != -1 {
		cmd.Dir = fsPath[:lastSlash]
	} else {
		// shouldn't happen
		cmd.Dir = docRoot
	}

	remoteAddr := ""
	remotePort := ""
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		remoteAddr, remotePort, _ = strings.Cut(tcpConn.RemoteAddr().String(), ":")
	}
	pathTranslated := ""
	if pathInfo != "" {
		pathTranslated = docRoot + pathInfo
	}
	cmd.Env = []string{
		"PATH=" + safePath,
		"GATEWAY_INTERFACE=CGI/1.1",                  // CGI
		"SERVER_PROTOCOL=GOPHER",                     // CGI
		"SERVER_SOFTWARE=" + serverSoftware,          // CGI
		"REQUEST_METHOD=GET",                         // CGI
		"PATH_INFO=" + pathInfo,                      // CGI
		"PATH_TRANSLATED=" + pathTranslated,          // CGI
		"SERVER_NAME=" + configString("serverhost"),  // CGI
		"SERVER_HOST=" + configString("serverhost"),  // Bucktooth
		"SERVER_PORT=" + configString("serverport"),  // CGI
		"QUERY_STRING=" + query,                      // CGI
		"REMOTE_ADDR=" + remoteAddr,                  // CGI
		"REMOTE_HOST=" + remoteAddr,                  // CGI
		"REMOTE_PORT=" + remotePort,                  // PyGopherd, Bucktooth
		"SCRIPT_NAME=" + scriptName,                  // CGI
		"SCRIPT_FILENAME=" + fsPath,                  // Apache, Gophernicus
		"GOPHER_SCRIPT_FILENAME=" + fsPath,           // port70
		"DOCUMENT_ROOT=" + docRoot,                   // Apache, Gophernicus
		"GOPHER_DOCUMENT_ROOT=" + docRoot,            // port70
		"SERVER_DESCRIPTION=" + configString("desc"), // Gophernicus
		"SEARCHREQUEST=" + search,                    // Gophernicus, PyGopherd, geomyidae
		"X_GOPHER_SEARCH=" + search,                  // geomyidae
		"QUERY_STRING_SEARCH=" + search,              // Motsognir
		"QUERY_STRING_URL=" + query,                  // Motsognir
		"SELECTOR=" + selector,                       // Gophernicus, PyGopherd, geomyidae, Bucktooth
		"GOPHER_DOCUMENT_SELECTOR=" + selector,       // port70
		"REQUEST=" + scriptName + pathInfo,           // Gophernicus, PyGopherd, geomyidae, Bucktooth
		fmt.Sprintf("THIRTEEN_UPTIME=%d", getUptime()),
		fmt.Sprintf("THIRTEEN_REQUESTS=%d", requestCount.Load()),
		fmt.Sprintf("THIRTEEN_BYTES=%d", bytesTransferred.Load()),

		// (XXX Bucktooth doesn't support PATH_INFO so it's not clear
		// whether REQUEST should include PATH_INFO or not)

		// TODO add other environment variables
	}

	reader, err := cmd.StdoutPipe()
	if err != nil {
		// XXX or other error?
		return makeErrorResponse(internalServerErrorError)
	}
	// TODO capture cmd's stderr and write it to an error log?

	err = cmd.Start()
	if err != nil {
		// XXX or other error?
		return makeErrorResponse(internalServerErrorError)
	}

	return response{reader, okStatus, cmd}
}

func getUptime() uint64 {
	return uint64(time.Since(startTime).Round(time.Second).Seconds())
}

func makeErrorResponse(e *responseError) response {
	return response{
		strings.NewReader(makeDirEntry('3', e.message, configString("serverhost"), configString("serverport"))),
		e.status,
		nil,
	}
}

// make a Gopher directory entry
func makeDirEntry(gtype byte, user string, host string, port string) string {
	return fmt.Sprintf("%c%s\t\t%s\t%s\r\n.\r\n", gtype, user, host, port)
}

// get the file system path to the file, the script name, and the path info
// corresponding to the given path
func splitPath(rootPath, path string) (fsPath, scriptName, pathInfo string, err *responseError) {
	// URL unescape path (so we can support a filename like "hello?")
	path, e := url.PathUnescape(path)
	if e != nil || strings.Contains(path, "\x00") {
		err = badRequestError
		return
	}

	path, ok := normalizePath(path)
	if !ok {
		err = forbiddenError
		return
	}

	return splitScriptPathAndPathInfo(rootPath+path, len(rootPath))
}

func splitScriptPathAndPathInfo(path string, startLength int) (fsPath, scriptName, pathInfo string, err *responseError) {
	isFile, _, _, pathErr := getStats(path)
	if pathErr == nil {
		// 1 If the given path exists and is a) a file or b) a directory with an index file (CGI or any other type) under it, use it directly (no path info). Return.
		if isFile {
			fsPath, scriptName = path, path[startLength:]
			return
		}
	}

	// default error
	err = fileNotFoundError

	// if CGIs are excluded, we cannot proceed any further (the user asked for it!)
	if excluded[cgiExt] {
		return
	}

	n, split := startLength, startLength

	// 2 Iterate over each path component (adding each path component to the current full path).
	for {
		curPath := path[:n]

		isFile, isDir, _, pathErr := getStats(curPath)

		// 3 If the current path is a file, use it. Return.
		if isFile {
			fsPath = curPath
			split = n
			break
		}

		// if the current path doesn't exist as a directory, there's no need to go further
		if pathErr != nil || !isDir {
			break
		}

		// 4 If the current path is a directory and contains an index file, remember the current path position and go back to 2.
		for _, indexPath := range indexPaths {
			pathWithIndex := curPath + indexPath
			isFile, _, _, pathErr = getStats(pathWithIndex)
			if pathErr == nil && isFile {
				fsPath = pathWithIndex
				split = n
				err = nil
			}
		}

		if n == len(path) {
			break
		}

		// get length of next path
		for n++; n != len(path) && path[n] != '/'; n++ {
		}

		if n == len(path) && path[n-1] == '/' {
			break
		}
	}
	scriptName, pathInfo = path[startLength:split], path[split:]
	return
}

func canServeFile(path string) bool { isFile, _, _, err := getStats(path); return isFile && err == nil }

func getStats(path string) (isFile, isDir, isCGI bool, responseErr *responseError) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			responseErr = fileNotFoundError
		} else {
			responseErr = forbiddenError
		}
		return
	}
	mode := fileInfo.Mode()
	needPerm := fs.FileMode(004)
	if mode.IsRegular() {
		if excluded[filepath.Ext(path)] {
			// this is not the path you're looking for
			responseErr = forbiddenError
			return
		}
		isFile = true
		if strings.HasSuffix(path, cgiExt) {
			isCGI = true
			needPerm = 005
		}
	} else if mode.IsDir() {
		isDir = true
		needPerm = 005
	}
	if mode&needPerm != needPerm {
		responseErr = forbiddenError
	}
	return
}

// Lexically normalize a path. Each component in the output string
// starts with a slash. The last component may be empty (represented by
// a trailing slash). Dot, dot-dot, and consecutive slashes are
// condensed. More dot-dots than preceding path components is an error
// (e.g., ".." or "x/../..").
func normalizePath(path string) (out string, ok bool) {
	newPath := make([]string, 0)

	for start, end := 0, 0; end < len(path); start = end + 1 {
		// find start of component (end of string or first
		// non-slash character, whichever comes first)
		for start < len(path) && path[start] == '/' {
			start++
		}

		// find end of component (end of string or first slash,
		// whichever comes first)
		end = start
		for end < len(path) && path[end] != '/' {
			end++
		}

		// start (inclusive) and end (exclusive) now delineate a
		// single path component without slashes. The last path
		// component may be empty.

		component := path[start:end]
		if component == ".." {
			// remove the last component from newPath
			if len(newPath) == 0 {
				// we can't go up from here!
				return
			}
			newPath = newPath[:len(newPath)-1]
		} else if component != "." {
			newPath = append(newPath, component)
		}
	}
	// flatten newPath to a string, with each component preceded by
	// a slash
	for i := range newPath {
		out += "/" + newPath[i]
	}
	ok = true
	return
}
