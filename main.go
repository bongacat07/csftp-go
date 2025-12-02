package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
)

var fileData []byte

// User data maps
var usernamesMap = map[int]string{
	1: "Vaibhav",
	2: "John",
	3: "Alice",
	4: "Bob",
	5: "Charlie",
}

var passwordsMap = map[int]string{
	1: "0000",
	2: "1234",
	3: "alicePass",
	4: "bobSecret",
	5: "charlie123",
}

// UUID → userID mapping
var uuidsMap = map[string]int{}

func main() {
	// Load example file
	data, err := os.ReadFile("lol.txt")
	if err != nil {
		log.Fatal(err)
	}
	fileData = data
	fmt.Println("Loaded file:", len(fileData), "bytes")

	// Start TCP server
	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatal("listen error:", err)
	}
	fmt.Println("Server listening on :8080")

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println("accept error:", err)
			continue
		}
		go handleConnection(conn)
	}
}

// Parse incoming request
func parser(request string) (string, []string) {
	slice := strings.Fields(request)
	if len(slice) < 1 {
		return "ERROR", nil
	}
	return slice[0], slice[1:]
}

// Handle a client connection
func handleConnection(conn net.Conn) {
	defer conn.Close()

	buf := make([]byte, 2048)
	n, err := conn.Read(buf)
	if err != nil {
		log.Println("read error:", err)
		return
	}

	request := string(buf[:n])
	fmt.Println("Client requested:", request)
	req, args := parser(request)
	handleRequest(req, args, conn)
}

// Dispatch requests to proper handler
func handleRequest(reqType string, args []string, conn net.Conn) {
	switch reqType {
	case "REGISTER":
		handleRegister(conn, args)
	case "AUTH":
		handleAuth(conn, args)
	case "GET":
		handleGet(conn, args)
	case "DELETE":
		handleDelete(conn, args)
	default:
		handleError(conn, "Invalid Request Method")
	}
}

// ======= HANDLERS =======

// Register a new user or return existing UUID
func handleRegister(conn net.Conn, args []string) {
	if len(args) < 2 {
		handleError(conn, "Usage: REGISTER username password")
		return
	}

	username := args[0]
	password := args[1]

	// Check if username exists
	for id, name := range usernamesMap {
		if name == username {
			// Username exists → check password
			if passwordsMap[id] == password {
				uuid := generateUUID(username, password)
				uuidsMap[uuid] = id
				conn.Write([]byte("200 OK\n" + uuid + "\n"))
			} else {
				conn.Write([]byte("401 Unauthorized\n"))
			}
			return
		}
	}

	// Username does not exist → create new
	newID := len(usernamesMap) + 1
	usernamesMap[newID] = username
	passwordsMap[newID] = password

	uuid := generateUUID(username, password)
	uuidsMap[uuid] = newID

	conn.Write([]byte("201 Created\n" + uuid + "\n"))
}

// Authenticate existing user
func handleAuth(conn net.Conn, args []string) {
	if len(args) < 2 {
		handleError(conn, "Usage: AUTH username password")
		return
	}

	username := args[0]
	password := args[1]

	for id, name := range usernamesMap {
		if name == username && passwordsMap[id] == password {
			uuid := generateUUID(username, password)
			uuidsMap[uuid] = id
			conn.Write([]byte("200 OK\n" + uuid + "\n"))
			return
		}
	}
	conn.Write([]byte("401 Unauthorized\n"))
}

// GET file (requires UUID)
func handleGet(conn net.Conn, args []string) {
	if len(args) < 2 {
		handleError(conn, "Usage: GET UUID filename")
		return
	}

	uuid := args[0]
	filename := args[1]

	if _, ok := uuidsMap[uuid]; !ok {
		conn.Write([]byte("401 Unauthorized\n"))
		return
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		conn.Write([]byte("404 File not found\n"))
		return
	}

	conn.Write(data)
}

// DELETE file (requires UUID)
func handleDelete(conn net.Conn, args []string) {
	if len(args) < 2 {
		handleError(conn, "Usage: DELETE UUID filename")
		return
	}

	uuid := args[0]
	filename := args[1]

	if _, ok := uuidsMap[uuid]; !ok {
		conn.Write([]byte("401 Unauthorized\n"))
		return
	}

	err := os.Remove(filename)
	if err != nil {
		conn.Write([]byte("404 File not found\n"))
		return
	}
	conn.Write([]byte("200 OK\n"))
}

// Error response
func handleError(conn net.Conn, msg string) {
	conn.Write([]byte("400 ERROR\n" + msg + "\n"))
}

// ======= UTIL =======
func generateUUID(username, password string) string {
	hasher := sha256.New()
	hasher.Write([]byte(username + password))
	return hex.EncodeToString(hasher.Sum(nil))
}
