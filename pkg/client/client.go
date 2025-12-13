package client

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
)

type Response struct {
	Status  uint8
	Message []byte
}

func StartClient() {
	//Start a client on any random port

	conn, err := net.Dial("tcp", ":8080")
	if err != nil {
		log.Fatal("failed to start client:", err)
	}
	log.Println("Client local address:", conn.LocalAddr())
	log.Println("Connected to server:", conn.RemoteAddr())
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		line := scanner.Text()
		method, arg := clientParser(line)
		handleMethod(method, arg, conn)
	}
}

func clientParser(line string) (string, string) {
	parts := strings.Fields(line)
	if len(parts) < 2 || len(parts) > 2 {
		return "error", ""
	}
	return parts[0], parts[1]
}
func handleMethod(method string, args string, conn net.Conn) {

	switch method {

	case "DELETE":
		reqDelete(args, conn)
	case "PUT":
		reqPut(args, conn)
	case "GET":
		reqGet(args, conn)

	}

}

func reqDelete(args string, conn net.Conn) {
	req := "DELETE " + args
	_, err := conn.Write([]byte(req))
	if err != nil {
		panic(err)
	}

	// Read the server response
	buf := make([]byte, 1024) // buffer size can be adjusted
	n, err := conn.Read(buf)
	if err != nil {
		panic(err)
	}

	// Convert received bytes into Response struct
	resp := Response{
		Status:  buf[0],
		Message: buf[1:n],
	}

	fmt.Printf("Server response: %d - %s\n", resp.Status, string(resp.Message))
}

func reqGet(args string, conn net.Conn) {
	req := "GET " + args
	_, err := conn.Write([]byte(req))
	if err != nil {
		panic(err)
	}

	file, err := os.Create(args)
	if err != nil {
		// Unable to create file

	}
	defer file.Close()
	bytesWritten, err := io.Copy(file, conn)
	log.Printf("Received file '%s' (%d bytes)", args, bytesWritten)

}

func reqPut(args string, conn net.Conn) {

	req := "PUT " + args
	_, err := conn.Write([]byte(req))
	if err != nil {
		panic(err)
	}
	file, err := os.Open(args)
	if err != nil { // File not found
		//
	}
	defer file.Close()

	// Stream file contents to the server
	// Indicate success with status 0
	bytesSent, err := io.Copy(conn, file)
	if err != nil {
		// Error during file transfer

	} else { // Successfully sent
		//
	}

	log.Printf("Sent file '%s' (%d bytes)", args, bytesSent)
}
