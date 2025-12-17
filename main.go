package main

import (
	"os"
<<<<<<< HEAD
	"path/filepath"
	"strings"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
=======

	"csftp/pkg/client"
	"csftp/pkg/server"
>>>>>>> main
)

func main() {
	if len(os.Args) < 2 {
		println("usage: csftp [server|client]")
		return
	}

	switch os.Args[1] {
	case "server":
		server.StartServer()
	case "client":
		client.StartClient()
	default:
		println("unknown command")
	}
}
<<<<<<< HEAD

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
		response := Response{Status: 63,
			Message: []byte("Unable to create file"),
		}
		buf := []byte{response.Status}
		buf = append(buf, response.Message...)
		conn.Write(buf)
		return

	}
	defer file.Close()
	v, _ := mem.VirtualMemory()                                  // See memory usage before receiving
	log.Printf("Available Memory: %v MB", v.Available/1024/1024) //log memory usage

	// Copy all incoming bytes from the connection into the file.
	// io.Copy reads until the client closes the connection.
	bytesWritten, err := io.Copy(file, conn)
	if err != nil {
		//error during file transfer
		response := Response{Status: 62, Message: []byte("PUT error")}
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
	file, err := os.Open(filename)
	if err != nil { // File not found
		response := Response{
			Status:  64,
			Message: []byte("file not found"),
		}
		buf := []byte{response.Status}
		buf = append(buf, response.Message...)
		conn.Write(buf)
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
	// PRINT PARAMETERS (just print, do not send)
	// ------------------------------------
	fmt.Println("=== Pre-Transfer Metrics ===")
	fmt.Printf("Available_Mem_MB: %.2f\n", float64(availableMemMB))
	fmt.Printf("Available_CPU_Percent: %.2f\n", cpuPercent)
	fmt.Printf("File_Size_Bytes: %d\n", filesize)
	fmt.Printf("File_Type: %s\n", fileType)

	// -----------------------------
	// TRANSFER
	// -----------------------------
	bytesSent, err := io.Copy(conn, file)
	if err != nil {
		// Error during file transfer
		response := Response{Status: 63, Message: []byte("GET error")}
		buf := []byte{response.Status}
		buf = append(buf, response.Message...)
		conn.Write(buf)
		log.Println("Send error:", err)
		return
	}

	// Successfully sent
	response := Response{Status: 69, Message: []byte("OK")}
	buf := []byte{response.Status}
	buf = append(buf, response.Message...)
	conn.Write(buf)

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
		response := Response{Status: 64,
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
	response := Response{Status: 65,
		Message: []byte(cmderror),
	}
	buf := []byte{response.Status}
	buf = append(buf, response.Message...)
	conn.Write(buf)
}
=======
>>>>>>> main
