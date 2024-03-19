package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)

var statusCodeToString = map[int]string{
	200: "OK",
	404: "Not Found",
}

type Request struct {
	Method string
	Path   string
}

type Response struct {
	StatusCode int
	Body       string
}

func (r Response) String() string {
	statusText := statusCodeToString[r.StatusCode]
	return fmt.Sprintf("HTTP/1.1 %d %s\r\n\r\n%s", r.StatusCode, statusText, r.Body)
}

func main() {
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

		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	stream := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {
			fmt.Println("Failed to close connection: ", err.Error())
		}
	}(conn)

	request, err := parseRequest(stream.Reader)
	if err != nil {
		fmt.Println("Failed to parse request: ", err.Error())
		os.Exit(1)
	}

	var response Response
	if request.Path == "/" {
		response = Response{StatusCode: 200}
	} else {
		response = Response{StatusCode: 404}
	}

	_, err = stream.WriteString(response.String())
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

func parseRequest(reader *bufio.Reader) (Request, error) {
	first, err := reader.ReadString('\n')
	if err != nil {
		return Request{}, err
	}

	parts := strings.Split(first, " ")
	return Request{Method: parts[0], Path: parts[1]}, nil
}
