package client

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"time"
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

	// Read the file size (8 bytes)
	buf := make([]byte, 8)
	_, err = io.ReadFull(conn, buf)
	if err != nil {
		log.Printf("Error reading file size: %v", err)
		return
	}
	fileSize := binary.BigEndian.Uint64(buf)

	// Allocate buffer to receive file data
	dataBuffer := make([]byte, fileSize)

	// Measure ONLY network transfer time
	start := time.Now()
	_, err = io.ReadFull(conn, dataBuffer)
	networkTime := time.Since(start)
	if err != nil {
		log.Printf("Error reading file data: %v", err)
		return
	}

	log.Printf("Received file '%s' (%d bytes) in %v", args, fileSize, networkTime)
	log.Printf("Network transfer time: %v", networkTime)

	// Read server response
	buff := make([]byte, 1024)
	resp, err := conn.Read(buff)
	if err != nil {
		panic(err)
	}

	response := Response{
		Status:  buff[0],
		Message: buff[1:resp],
	}
	fmt.Printf("Server response: %d - %s\n", response.Status, string(response.Message))

	// TODO: Log networkTime to CSV file with transfer_id
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
	fi, err := file.Stat()
	if err != nil {
		//
	}
	fileSize := fi.Size()
	fmt.Println("File size:", fileSize)
	buf := make([]byte, 8) // 8 bytes for uint64
	binary.BigEndian.PutUint64(buf, uint64(fileSize))
	conn.Write(buf)

	bytesSent, err := io.Copy(conn, file)
	// Stream file contents to the server
	// Indicate success with status 0

	if err != nil {
		// Error during file transfer

	} else { // Successfully sent
		//
	}
	buff := make([]byte, 1024) // buffer size can be adjusted
	n, err := conn.Read(buff)
	if err != nil {
		panic(err)
	}

	// Convert received bytes into Response struct
	resp := Response{
		Status:  buff[0],
		Message: buff[1:n],
	}

	fmt.Printf("Server response: %d - %s\n", resp.Status, string(resp.Message))
	log.Printf("Sent file '%s' (%d bytes)", args, bytesSent)
}
