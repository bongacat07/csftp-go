package server

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"os"
	"path/filepath"
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
	curState := stateSendL1
	cpuLoad, _ := cpu.Percent(time.Second, false)
	baselineCPU := cpuLoad[0]
	fileData, _ := os.ReadFile(filename)
	fileSize := len(fileData)
	fileType := filepath.Ext(filename)
	// Memory
	vm, _ := mem.VirtualMemory()
	baselineMem := vm.Available / (1024 * 1024)
	fmt.Println("=== Pre-Compression Metrics ===")
	fmt.Printf("File_Size_Bytes: %d\n", fileSize)
	fmt.Printf("File_Type: %s\n", fileType)
	fmt.Printf("CPU_Percent: %.2f\n", baselineCPU)
	fmt.Printf("Available_Mem_MB: %d\n", baselineMem)
name:
	for {
		switch curState {
		case stateSendL1:
			headerSize1, compressedData1 := compressor(1, fileData)
			writeToConn(headerSize1, conn, compressedData1)

			curState = stateWaitACK1
		case stateSendL2:
			headerSize2, compressedData2 := compressor(3, fileData)
			writeToConn(headerSize2, conn, compressedData2)
			curState = stateWaitACK2
		case stateSendL3:
			headerSize3, compressedData3 := compressor(9, fileData)
			writeToConn(headerSize3, conn, compressedData3)
			curState = stateWaitACK3

		case stateWaitACK1:
			if readACK(conn) {
				waitForSystemSettle(baselineCPU, baselineMem)
				curState = stateSendL2
			}

		case stateWaitACK2:
			if readACK(conn) {
				waitForSystemSettle(baselineCPU, baselineMem)
				curState = stateSendL3
			}
		case stateWaitACK3:
			if readACK(conn) {
				curState = stateSendResponse
			}
		case stateSendResponse:
			sendResponse(conn)
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

func compressor(level int, fileData []byte) ([]byte, []byte) {

	// CPU (use 1 second sample for accuracy)

	var compressedBuffer bytes.Buffer
	start := time.Now()
	gzipWriter, _ := gzip.NewWriterLevel(&compressedBuffer, level)
	_, err := gzipWriter.Write(fileData)
	if err != nil {
		log.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil { // ← ADD THIS BACK
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
		message := []byte("Transfer error")
		responseSize := uint16(1 + len(message))

		respBuf := make([]byte, 2+1+len(message))
		binary.BigEndian.PutUint16(respBuf[0:2], responseSize)
		respBuf[2] = 70
		copy(respBuf[3:], message)

		conn.Write(respBuf)
		return
	}
}

func sendResponse(conn net.Conn) {
	message := []byte("OK")
	responseSize := uint16(1 + len(message))

	buf := make([]byte, 2+1+len(message))
	binary.BigEndian.PutUint16(buf[0:2], responseSize)
	buf[2] = 69
	copy(buf[3:], message)

	conn.Write(buf)
	log.Println("Success")
}

func waitForSystemSettle(baselineCPU float64, baselineMem uint64) {
	fmt.Println("Waiting for system to settle...")

	for {
		time.Sleep(500 * time.Millisecond)

		cpuLoad, _ := cpu.Percent(200*time.Millisecond, false)
		currentCPU := cpuLoad[0]

		vm, _ := mem.VirtualMemory()
		currentMem := vm.Available / (1024 * 1024)

		// Calculate percentage differences
		cpuDiff := math.Abs(currentCPU - baselineCPU)
		memDiffPercent := math.Abs(float64(currentMem)-float64(baselineMem)) / float64(baselineMem) * 100

		// Both within 1%
		if cpuDiff < 1.0 && memDiffPercent < 1.0 {
			fmt.Println("System settled.")
			return
		}

		fmt.Printf("Settling... CPU diff: %.2f%%, Mem diff: %.2f%%\n",
			cpuDiff, memDiffPercent)
	}
}
