package server

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"
)

type Response struct {
	Status  uint8
	Message []byte
}

func StartServer() {
	// Start a TCP listener on port 8080.
	// ln is a listening socket that accepts incoming client connections.
	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatal("failed to start server:", err)
	}
	defer ln.Close()

	log.Println("CSFTP Server started on :8080")

	// Main server loop: accept and process client connections
	for {
		conn, err := ln.Accept()
		if err != nil {
			// Accept errors can occur due to transient network issues.
			log.Printf("accept failed: %v", err)
			continue
		}

		// Each client is handled concurrently so multiple transfers can occur.
		go handleConnection(conn)
	}
}

// handleConnection processes a single client connection end-to-end.
// It reads a request line, parses it into method + argument, and
// dispatches to the correct handler for the file operation.
func handleConnection(conn net.Conn) {
	defer conn.Close()

	for {
		// 60 second timeout between requests
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		req, arg, err := parser(conn)
		if err != nil {
			if err != io.EOF {
				log.Println("parse error:", err)
			}
			return
		}

		handleRequest(req, arg, conn)
	}
}

func parser(r io.Reader) (string, string, error) {
	// Read length
	lenBuf := make([]byte, 2)
	if _, err := io.ReadFull(r, lenBuf); err != nil {
		return "", "", err
	}
	filenameLength := binary.BigEndian.Uint16(lenBuf)

	// Read opcode
	opBuf := make([]byte, 1)
	if _, err := io.ReadFull(r, opBuf); err != nil {
		return "", "", err
	}

	var reqMethod string
	switch opBuf[0] {
	case 0x01:
		reqMethod = "GET"
	case 0x02:
		reqMethod = "PUT"
	case 0x03:
		reqMethod = "DELETE"
	default:
		return "", "", fmt.Errorf("invalid opcode: %d", opBuf[0])
	}

	// Read filename
	nameBuf := make([]byte, filenameLength)
	if _, err := io.ReadFull(r, nameBuf); err != nil {
		return "", "", err
	}

	return reqMethod, string(nameBuf), nil
}

// handleRequest routes the parsed request to the correct handler.
func handleRequest(reqType string, arg string, conn net.Conn) {
	switch reqType {
	case "PUT":
		handlePut(conn, arg)
	case "GET":
		handleGet(conn, arg)
	case "DELETE":
		handleDelete(conn, arg)
	default:
		handleError(conn, "Invalid Request Method")
	}
}

// handlePut receives a file from the client and writes it to disk.
// Protocol:
//
//	Client sends: "PUT filename.ext"
//	Then immediately streams raw file bytes until EOF or disconnect.
func handlePut(conn net.Conn, filename string) {
	// Create the file on the server.
	file, err := os.Create(filename)
	if err != nil {
		// Unable to create file
		message := []byte("Unable to create file")
		responseSize := uint16(1 + len(message))

		buf := make([]byte, 2+1+len(message))
		binary.BigEndian.PutUint16(buf[0:2], responseSize)
		buf[2] = 62
		copy(buf[3:], message)

		conn.Write(buf)
		return
	}
	defer file.Close()

	buf := make([]byte, 8)
	_, errr := io.ReadFull(conn, buf)
	if errr != nil {
		message := []byte("Buffer error")
		responseSize := uint16(1 + len(message))

		buf := make([]byte, 2+1+len(message))
		binary.BigEndian.PutUint16(buf[0:2], responseSize)
		buf[2] = 63
		copy(buf[3:], message)

		conn.Write(buf)
		return
	}

	n := binary.BigEndian.Uint64(buf)
	// Copy all incoming bytes from the connection into the file.
	// io.Copy reads until the client closes the connection.
	bytesWritten, err := io.CopyN(file, conn, int64(n))
	if err != nil {
		//error during file transfer
		message := []byte("PUT error")
		responseSize := uint16(1 + len(message))

		buf := make([]byte, 2+1+len(message))
		binary.BigEndian.PutUint16(buf[0:2], responseSize)
		buf[2] = 64
		copy(buf[3:], message)

		conn.Write(buf)
		return
	} else {
		// Successfully sent
		message := []byte("OK")
		responseSize := uint16(1 + len(message))

		buf := make([]byte, 2+1+len(message))
		binary.BigEndian.PutUint16(buf[0:2], responseSize)
		buf[2] = 69
		copy(buf[3:], message)

		conn.Write(buf)
	}

	log.Printf("Received file '%s' (%d bytes)", filename, bytesWritten)
}

// handleGet sends a requested file back to the client.
// Protocol:
//
//	Client sends: "GET filename.ext"
//	Server sends raw file bytes.
func handleGet(conn net.Conn, filename string) {
	file, err := os.Open(filename)
	if err != nil {
		// File not found
		message := []byte("file not found")
		responseSize := uint16(1 + len(message))

		buf := make([]byte, 2+1+len(message))
		binary.BigEndian.PutUint16(buf[0:2], responseSize)
		buf[2] = 65
		copy(buf[3:], message)

		conn.Write(buf)
		return
	}
	defer file.Close()

	// Get file info
	fi, err := file.Stat()
	if err != nil {
		message := []byte("File info error")
		responseSize := uint16(1 + len(message))

		buf := make([]byte, 2+1+len(message))
		binary.BigEndian.PutUint16(buf[0:2], responseSize)
		buf[2] = 66
		copy(buf[3:], message)

		conn.Write(buf)
		return
	}

	// File size in bytes
	fileSize := fi.Size()
	fmt.Println("File size:", fileSize)
	buf := make([]byte, 8) // 8 bytes for uint64
	binary.BigEndian.PutUint64(buf, uint64(fileSize))
	conn.Write(buf)

	bytesSent, err := io.Copy(conn, file)

	if err != nil {
		// Error during file transfer
		message := []byte("GET error")
		responseSize := uint16(1 + len(message))

		buf := make([]byte, 2+1+len(message))
		binary.BigEndian.PutUint16(buf[0:2], responseSize)
		buf[2] = 70
		copy(buf[3:], message)

		conn.Write(buf)
		return
	} else {
		// Successfully sent
		message := []byte("OK")
		responseSize := uint16(1 + len(message))

		buf := make([]byte, 2+1+len(message))
		binary.BigEndian.PutUint16(buf[0:2], responseSize)
		buf[2] = 69
		copy(buf[3:], message)

		conn.Write(buf)
	}

	log.Printf("Sent file '%s' (%d bytes)", filename, bytesSent)
}

// handleDelete removes a file from the server filesystem.
// Protocol:
//
//	Client sends: "DELETE filename.ext"
func handleDelete(conn net.Conn, filename string) {
	err := os.Remove(filename)
	if err != nil {
		// File not found or unable to delete
		message := []byte("file not found")
		responseSize := uint16(1 + len(message))

		buf := make([]byte, 2+1+len(message))
		binary.BigEndian.PutUint16(buf[0:2], responseSize)
		buf[2] = 65
		copy(buf[3:], message)

		conn.Write(buf)
		return
	} else {
		// Successfully deleted
		message := []byte("OK")
		responseSize := uint16(1 + len(message))

		buf := make([]byte, 2+1+len(message))
		binary.BigEndian.PutUint16(buf[0:2], responseSize)
		buf[2] = 69
		copy(buf[3:], message)

		conn.Write(buf)
	}

	log.Printf("Deleted file '%s'", filename)
}

// handleError sends an error message to the client.
func handleError(conn net.Conn, msg string) {
	cmderror := msg + "Invalid Request Method"
	message := []byte(cmderror)
	responseSize := uint16(1 + len(message))

	buf := make([]byte, 2+1+len(message))
	binary.BigEndian.PutUint16(buf[0:2], responseSize)
	buf[2] = 68
	copy(buf[3:], message)

	conn.Write(buf)
}
