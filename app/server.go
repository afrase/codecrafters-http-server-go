package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var statusCodeToString = map[int]string{
	200: "OK",
	201: "Created",
	404: "Not Found",
	405: "Method Not Allowed",
	500: "Internal Server Error",
}

type Request struct {
	Method  string
	Path    string
	Headers map[string]string
	Body    string
}

type Response struct {
	StatusCode int
	Headers    map[string]string
	Body       string
}

func (r Response) String() string {
	statusText, ok := statusCodeToString[r.StatusCode]
	if !ok {
		statusText = "Unknown"
	}

	// No headers so assume plain text result.
	if r.Headers == nil {
		r.Headers = make(map[string]string)
	}

	// Make sure content-type is always set.
	if _, ok = r.Headers["Content-Type"]; !ok {
		r.Headers["Content-Type"] = "text/plain"
	}

	// Figure out content length if not set.
	if _, ok = r.Headers["Content-Length"]; !ok {
		r.Headers["Content-Length"] = strconv.Itoa(len(r.Body))
	}

	var headerString strings.Builder
	for k, v := range r.Headers {
		headerString.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}

	return fmt.Sprintf("HTTP/1.1 %d %s\r\n%s\r\n%s", r.StatusCode, statusText, headerString.String(), r.Body)
}

func main() {
	var dir string
	if len(os.Args) > 1 && os.Args[1] == "--directory" {
		dir = os.Args[2]
	}

	l, err := net.Listen("tcp", ":4221")
	if err != nil {
		fmt.Println("Failed to bind to port 4221")
		os.Exit(1)
	}

	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting connection: ", err.Error())
			os.Exit(1)
		}

		// Spawn a go thread
		go handleConnection(conn, dir)
	}
}

func handleConnection(conn net.Conn, dir string) {
	stream := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {
			fmt.Println("Failed to close connection: ", err.Error())
		}
	}(conn)

	resp := Response{StatusCode: 200}
	req, err := parseRequest(stream.Reader)
	if err != nil {
		fmt.Println("Failed to parse req: ", err.Error())
		os.Exit(1)
	}

	if strings.HasPrefix(req.Path, "/echo") {
		pathParts := strings.SplitN(req.Path, "/echo/", 2)
		resp.Body = pathParts[1]
	} else if req.Path == "/user-agent" {
		userAgent := req.Headers["User-Agent"]
		resp.Body = userAgent
	} else if strings.HasPrefix(req.Path, "/files") && dir != "" {
		pathParts := strings.SplitN(req.Path, "/files/", 2)
		fileName := pathParts[1]
		path := filepath.Join(dir, fileName)
		switch req.Method {
		case "GET":
			handleFileGet(path, &resp)
		case "POST":
			handleFilePost(path, &req, &resp)
		default:
			resp.Headers = map[string]string{"Allow": "GET, POST"}
			resp.StatusCode = 405
		}
	} else if req.Path != "/" {
		resp.StatusCode = 404
	}

	_, err = stream.WriteString(resp.String())
	if err != nil {
		fmt.Println("Failed to write to socket: ", err.Error())
		os.Exit(1)
	}

	err = stream.Flush()
	if err != nil {
		fmt.Println("Failed to flush to socket")
		os.Exit(1)
	}
}

func handleFilePost(path string, req *Request, resp *Response) {
	file, err := os.Create(path)
	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	if err != nil {
		resp.StatusCode = 500
		fmt.Println("failed to create file: ", err.Error())
		return
	}

	_, err = file.WriteString(req.Body)
	if err != nil {
		resp.StatusCode = 500
		fmt.Println("failed to write file: ", err.Error())
		return
	}

	err = file.Sync()
	if err != nil {
		resp.StatusCode = 500
		fmt.Println("failed to commit file: ", err.Error())
		return
	}

	resp.StatusCode = 201
}

func handleFileGet(path string, resp *Response) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		resp.StatusCode = 404
		return
	}

	file, err := os.Open(path)
	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	if err != nil {
		resp.StatusCode = 500
		fmt.Println("failed to open file: ", err.Error())
		return
	}

	all, err := io.ReadAll(file)
	if err != nil {
		resp.StatusCode = 500
		fmt.Println("failed to read file: ", err.Error())
		return
	}

	resp.Body = string(all)
	resp.Headers = map[string]string{
		"Content-Type": "application/octet-stream",
	}
}

func parseRequest(reader *bufio.Reader) (Request, error) {
	request := Request{
		Headers: make(map[string]string),
	}

	firstLine, err := reader.ReadString('\n')
	if err != nil {
		return Request{}, fmt.Errorf("malformed HTTP request")
	}
	parts := strings.Split(firstLine, " ")
	request.Method = parts[0]
	request.Path = parts[1]

	for {
		curLine, err := reader.ReadString('\n')
		if curLine == "\r\n" {
			break
		}
		if err == io.EOF {
			return request, nil
		} else if err != nil {
			return Request{}, err
		}

		headerParts := strings.SplitN(curLine, ":", 2)
		request.Headers[headerParts[0]] = strings.TrimSpace(headerParts[1])
	}

	// If the content length is set read the body.
	contentLenStr, ok := request.Headers["Content-Length"]
	if !ok {
		return request, nil
	}

	contentLen, _ := strconv.Atoi(contentLenStr)
	buf := make([]byte, contentLen)
	// This probably should read in chunks.
	_, err = io.ReadFull(reader, buf)
	if err != nil {
		return request, err
	}
	request.Body = string(buf)

	return request, nil
}
