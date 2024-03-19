package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
)

func main() {
	l, err := net.Listen("tcp", "0.0.0.0:4221")
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
			fmt.Println("Failed to close connection")
		}
	}(conn)

	_, err := stream.WriteString("HTTP/1.1 200 OK\r\n\r\n")
	if err != nil {
		fmt.Println("Failed to write to socket")
		os.Exit(1)
	}

	err = stream.Flush()
	if err != nil {
		fmt.Println("Failed to flush to socket")
		os.Exit(1)
	}
}
