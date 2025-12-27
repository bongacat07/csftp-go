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
	filenameBytes := []byte(args)
	filenameLen := uint16(len(filenameBytes))

	// Allocate exact size
	reqBuf := make([]byte, 2+1+len(filenameBytes))
	offset := 0

	// filename length (2 bytes)
	binary.BigEndian.PutUint16(reqBuf[offset:], filenameLen)
	offset += 2

	// opcode
	reqBuf[offset] = byte(3) // DELETE opcode
	offset++

	// filename bytes
	copy(reqBuf[offset:], filenameBytes)

	_, error := conn.Write(reqBuf)
	if error != nil {
		panic(error)
	}

	// Read the server response with framing
	sizeBuf := make([]byte, 2)
	_, err := io.ReadFull(conn, sizeBuf)
	if err != nil {
		panic(err)
	}
	responseSize := binary.BigEndian.Uint16(sizeBuf)

	statusBuf := make([]byte, 1)
	_, err = io.ReadFull(conn, statusBuf)
	if err != nil {
		panic(err)
	}

	messageBuf := make([]byte, responseSize-1)
	_, err = io.ReadFull(conn, messageBuf)
	if err != nil {
		panic(err)
	}

	resp := Response{
		Status:  statusBuf[0],
		Message: messageBuf,
	}

	fmt.Printf("Server response: %d - %s\n", resp.Status, string(resp.Message))
}

func reqGet(args string, conn net.Conn) {
	// Send GET request
	filenameBytes := []byte(args)
	filenameLen := uint16(len(filenameBytes))

	reqBuf := make([]byte, 2+1+len(filenameBytes))
	binary.BigEndian.PutUint16(reqBuf[0:], filenameLen)
	reqBuf[2] = byte(1) // GET opcode
	copy(reqBuf[3:], filenameBytes)

	_, err := conn.Write(reqBuf)
	if err != nil {
		panic(err)
	}

	fmt.Println("=== Transfer Time Measurements ===")

	// Receive 3 compressed versions and measure transfer time for each
	for level := 1; level <= 3; level++ {
		// Read compressed size header (8 bytes)
		sizeBuf := make([]byte, 8)
		_, err := io.ReadFull(conn, sizeBuf)
		if err != nil {
			panic(err)
		}
		compressedSize := binary.BigEndian.Uint64(sizeBuf)

		// Create buffer for compressed data
		buffer := make([]byte, compressedSize)

		// Start measuring transfer time
		transferStart := time.Now()

		// Read compressed data into buffer
		_, err = io.ReadFull(conn, buffer)
		if err != nil {
			panic(err)
		}

		// Stop measuring transfer time
		transferTime := time.Since(transferStart)

		// Print metrics
		fmt.Printf("Level_%d_Compressed_Size: %d bytes\n", level, compressedSize)
		fmt.Printf("Level_%d_Transfer_Time: %v\n", level, transferTime)

		// Clear buffer to free memory
		buffer = nil

		// Send ACK to server
		ackBuf := []byte{7, 0}
		_, err = conn.Write(ackBuf)
		if err != nil {
			panic(err)
		}
	}

	// Read final response with binary framing
	sizeBuf := make([]byte, 2)
	_, err = io.ReadFull(conn, sizeBuf)
	if err != nil {
		panic(err)
	}
	responseSize := binary.BigEndian.Uint16(sizeBuf)

	statusBuf := make([]byte, 1)
	_, err = io.ReadFull(conn, statusBuf)
	if err != nil {
		panic(err)
	}

	messageBuf := make([]byte, responseSize-1)
	_, err = io.ReadFull(conn, messageBuf)
	if err != nil {
		panic(err)
	}

	response := Response{
		Status:  statusBuf[0],
		Message: messageBuf,
	}
	fmt.Printf("Server response: %d - %s\n", response.Status, string(response.Message))
	log.Printf("GET complete for '%s'", args)
}
func reqPut(args string, conn net.Conn) {

	filenameBytes := []byte(args)
	filenameLen := uint16(len(filenameBytes))

	// Allocate exact size
	reqBuf := make([]byte, 2+1+len(filenameBytes))
	offset := 0

	// filename length (2 bytes)
	binary.BigEndian.PutUint16(reqBuf[offset:], filenameLen)
	offset += 2

	// opcode
	reqBuf[offset] = byte(2) // PUT opcode
	offset++

	// filename bytes
	copy(reqBuf[offset:], filenameBytes)

	_, error := conn.Write(reqBuf)
	if error != nil {
		panic(error)
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

	// Read the server response with framing
	sizeBuf := make([]byte, 2)
	_, err = io.ReadFull(conn, sizeBuf)
	if err != nil {
		panic(err)
	}
	responseSize := binary.BigEndian.Uint16(sizeBuf)

	statusBuf := make([]byte, 1)
	_, err = io.ReadFull(conn, statusBuf)
	if err != nil {
		panic(err)
	}

	messageBuf := make([]byte, responseSize-1)
	_, err = io.ReadFull(conn, messageBuf)
	if err != nil {
		panic(err)
	}

	resp := Response{
		Status:  statusBuf[0],
		Message: messageBuf,
	}

	fmt.Printf("Server response: %d - %s\n", resp.Status, string(resp.Message))
	log.Printf("Sent file '%s' (%d bytes)", args, bytesSent)
}
