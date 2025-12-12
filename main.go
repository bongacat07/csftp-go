package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
)

// main starts the TCP server and listens for incoming connections.

func main() {
	// Start a TCP listener on port 8080.
	// ln is a listening socket that accepts incoming client connections.
	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatal("failed to start listener:", err)
	}
	defer ln.Close()

	log.Println("Server started on :8080")

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

	// Read up to 2048 bytes, enough for the request line (e.g. "PUT file.txt")
	buf := make([]byte, 2048)
	n, err := conn.Read(buf)
	if err != nil {
		log.Println("read error:", err)
		return
	}

	// Extract only the bytes actually read
	request := string(buf[:n])
	fmt.Println("Client requested:", request)

	req, arg := parser(request)
	handleRequest(req, arg, conn)
}

// parser splits the client request into a command and an argument.
// Example input: "PUT hello.txt"
// Returns: ("PUT", "hello.txt")
func parser(request string) (string, string) {
	parts := strings.Fields(request)
	if len(parts) < 2 {
		return "ERROR", ""
	}
	return parts[0], parts[1]
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
		handleError(conn, "Failed to create file")
		return
	}
	defer file.Close()
	v, _ := mem.VirtualMemory()                                  // See memory usage before receiving
	log.Printf("Available Memory: %v MB", v.Available/1024/1024) //log memory usage

	// Copy all incoming bytes from the connection into the file.
	// io.Copy reads until the client closes the connection.
	bytesWritten, err := io.Copy(file, conn)
	if err != nil {
		log.Printf("PUT error writing file: %v", err)
		handleError(conn, "Failed to receive file")
		return
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
		handleError(conn, "File not found")
		return
	}
	defer file.Close()

	// FILE INFO
	info, _ := file.Stat()
	filesize := info.Size()
	fileType := filepath.Ext(filename)

	// MEMORY
	vm, _ := mem.VirtualMemory()
	availableMemMB := vm.Available / (1024 * 1024)

	// CPU
	cpuLoad, _ := cpu.Percent(0, false)
	cpuPercent := cpuLoad[0]

	// ------------------------------------
	// PRINT PARAMETERS (your 7 values)
	// ------------------------------------
	fmt.Println("=== Pre-Transfer Metrics ===")
	fmt.Printf("Available_Mem_MB: %.2f\n", float64(availableMemMB))
	fmt.Printf("Available_CPU_Percent: %.2f\n", cpuPercent)
	fmt.Printf("File_Size_Bytes: %d\n", filesize)
	fmt.Printf("File_Type: %s\n", fileType)

	// -----------------------------
	// TRANSFER
	// -----------------------------
	_, err = io.Copy(conn, file)
	if err != nil {
		log.Println("Send error:", err)
	}
}

// handleDelete removes a file from the server filesystem.
// Protocol:
//
//	Client sends: "DELETE filename.ext"
func handleDelete(conn net.Conn, filename string) {
	err := os.Remove(filename)
	if err != nil {
		handleError(conn, "Failed to delete file (not found)")
		return
	}

	fmt.Fprintf(conn, "OK: Deleted %s\n", filename)
	log.Printf("Deleted file '%s'", filename)
}

// handleError sends an error message to the client.
func handleError(conn net.Conn, msg string) {
	fmt.Fprintf(conn, "ERROR: %s\n", msg)
}
