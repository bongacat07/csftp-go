package server

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
)

type Response struct {
	Status  uint8
	Message []byte
}
type state int

const (
	stateSendL1 state = iota
	stateWaitACK1
	stateSendL2
	stateWaitACK2
	stateSendL3
	stateWaitACK3
	stateSendResponse
)

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
	defer conn.Close() // Keep this, but close AFTER the loop

	// Keep reading requests on same connection
	for {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		buf := make([]byte, 2048)
		n, err := conn.Read(buf)
		if err != nil {
			// Connection closed or error - exit loop
			if err != io.EOF {
				log.Println("read error:", err)
			}
			return
		}

		request := string(buf[:n])
		fmt.Println("Client requested:", request)

		req, arg := parser(request)
		handleRequest(req, arg, conn)

		// TODO: Check if client sent close signal
		// if req.wantsClose { break }
	}
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
		// Unable to create file
		response := Response{Status: 62,
			Message: []byte("Unable to create file"),
		}
		buf := []byte{response.Status}
		buf = append(buf, response.Message...)
		conn.Write(buf)
		return

	}
	defer file.Close()
	buf := make([]byte, 8)
	_, errr := io.ReadFull(conn, buf)
	if errr != nil {
		response := Response{Status: 63,
			Message: []byte("Buffer error"),
		}
		buf := []byte{response.Status}
		buf = append(buf, response.Message...)
		conn.Write(buf)
	}
	n := binary.BigEndian.Uint64(buf)
	// Copy all incoming bytes from the connection into the file.
	// io.Copy reads until the client closes the connection.
	bytesWritten, err := io.CopyN(file, conn, int64(n))
	if err != nil {
		//error during file transfer
		response := Response{Status: 64, Message: []byte("PUT error")}
		buf := []byte{response.Status}
		buf = append(buf, response.Message...)
		conn.Write(buf)
		return
	} else { // Successfully sent
		response := Response{Status: 69, Message: []byte("OK")}
		buf := []byte{response.Status}
		buf = append(buf, response.Message...)
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
	curState := stateSendL1

name:
	for {
		switch curState {
		case stateSendL1:
			headerSize1, compressedData1 := compressor(1, filename)
			writeToConn(headerSize1, conn, compressedData1)
			curState = stateWaitACK1
		case stateSendL2:
			headerSize2, compressedData2 := compressor(3, filename)
			writeToConn(headerSize2, conn, compressedData2)
			curState = stateWaitACK2
		case stateSendL3:
			headerSize3, compressedData3 := compressor(9, filename)
			writeToConn(headerSize3, conn, compressedData3)
			curState = stateWaitACK3

		case stateWaitACK1:
			if readACK(conn) {
				curState = stateSendL2
			}

		case stateWaitACK2:
			if readACK(conn) {
				curState = stateSendL3
			}
		case stateWaitACK3:
			if readACK(conn) {
				curState = stateSendResponse
			}
		case stateSendResponse:
			sendRespone(conn)
			break name
		}

	}

	// TODO: Log to CSV here
}

// handleDelete removes a file from the server filesystem.
// Protocol:
//
//	Client sends: "DELETE filename.ext"
func handleDelete(conn net.Conn, filename string) {
	err := os.Remove(filename)
	if err != nil {
		// File not found or unable to delete
		response := Response{Status: 65,
			Message: []byte("file not found"),
		}
		buf := []byte{response.Status}
		buf = append(buf, response.Message...)
		conn.Write(buf)
		return
	} else {
		// Successfully deleted
		response := Response{Status: 69,
			Message: []byte("OK"),
		}
		buf := []byte{response.Status}
		buf = append(buf, response.Message...)
		conn.Write(buf)
	}

	log.Printf("Deleted file '%s'", filename)
}

// handleError sends an error message to the client.
func handleError(conn net.Conn, msg string) {
	cmderror := msg + "Invalid Request Method"
	response := Response{Status: 68,
		Message: []byte(cmderror),
	}
	buf := []byte{response.Status}
	buf = append(buf, response.Message...)
	conn.Write(buf)
}

func compressor(level int, filename string) ([]byte, []byte) {
	fileData, err := os.ReadFile(filename)
	fileSize := len(fileData)
	fileType := filepath.Ext(filename)

	// CPU (use 1 second sample for accuracy)
	cpuLoad, _ := cpu.Percent(time.Second, false)
	cpuPercent := cpuLoad[0]

	// Memory
	vm, _ := mem.VirtualMemory()
	availableMemMB := vm.Available / (1024 * 1024)

	fmt.Println("=== Pre-Compression Metrics ===")
	fmt.Printf("File_Size_Bytes: %d\n", fileSize)
	fmt.Printf("File_Type: %s\n", fileType)
	fmt.Printf("CPU_Percent: %.2f\n", cpuPercent)
	fmt.Printf("Available_Mem_MB: %d\n", availableMemMB)

	var compressedBuffer bytes.Buffer
	start := time.Now()
	gzipWriter, _ := gzip.NewWriterLevel(&compressedBuffer, level)
	_, err = gzipWriter.Write(fileData)
	if err != nil {
		log.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		log.Fatal(err)
	}
	compressionTime := time.Since(start)

	compressedData := compressedBuffer.Bytes()
	compressedSize := len(compressedData)

	fmt.Printf("Compression_Time: %v\n", compressionTime)
	fmt.Printf("Compressed_Size: %d\n", compressedSize)
	headerSize := makeHeader(compressedSize)

	return headerSize, compressedData

}
func makeHeader(sizeaftc int) []byte {
	sizeBuf := make([]byte, 8)
	binary.BigEndian.PutUint64(sizeBuf, uint64(sizeaftc))
	return sizeBuf
}

func writeToConn(buf []byte, conn net.Conn, compData []byte) {
	if _, err := conn.Write(buf); err != nil {
		return
	}

	if _, err := conn.Write(compData); err != nil {
		response := Response{
			Status:  70,
			Message: []byte("Transfer error"),
		}

		respBuf := []byte{response.Status}
		respBuf = append(respBuf, response.Message...)
		conn.Write(respBuf)
		return
	}
}
func sendRespone(conn net.Conn) {
	response := Response{Status: 69, Message: []byte("OK")}
	buf := []byte{response.Status}
	buf = append(buf, response.Message...)
	conn.Write(buf)
	log.Println("Success")
}
func readACK(conn net.Conn) bool {
	buf := make([]byte, 2)
	_, err := io.ReadFull(conn, buf)

	if err != nil {
		panic(err)
	}

	if buf[0] == 7 {
		return true
	} else {
		return false
	}
}
