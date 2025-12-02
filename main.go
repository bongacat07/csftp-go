package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

var home string
var credentialsPath string
var uuidsMap = map[string]string{} // uuid → username

func init() {
	h, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	home = h
	credentialsPath = home + "/.csftp/credentials"
}

// Credentials struct
type Credentials struct {
	Username string `json:"username"`
	UUID     string `json:"uuid"`
	Hash     string `json:"hash"`
}

const credentialsFile = "credentials.json"

func main() {

	// Create credentials directory if it doesn't exist
	err := os.MkdirAll(credentialsPath, 0700)
	if err != nil {
		log.Fatal(err)
	}

	// Load existing UUID → username mapping at startup
	loadUUIDs()

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

// ======= REGISTER =======
func handleRegister(conn net.Conn, args []string) {
	if len(args) < 2 {
		handleError(conn, "Usage: REGISTER username password")
		return
	}

	username := args[0]
	password := args[1]

	// Load existing credentials if any
	creds := loadCredentials()

	// Check if user already exists
	if user, ok := creds[username]; ok {
		conn.Write([]byte("200 OK\n" + user.UUID + "\n")) // Return existing UUID
		uuidsMap[user.UUID] = username                    // Add to session map
		return
	}

	// Generate UUID for the new user
	uuid := generateUUID(username, password)

	// Hash the password using bcrypt
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		handleError(conn, "Error hashing password")
		return
	}

	// Save new user
	creds[username] = Credentials{
		Username: username,
		UUID:     uuid,
		Hash:     string(hashedPassword),
	}

	saveCredentials(creds)

	// Add to UUID → username map
	uuidsMap[uuid] = username

	conn.Write([]byte("201 Created\n" + uuid + "\n"))
}

// ======= AUTH =======
func handleAuth(conn net.Conn, args []string) {
	if len(args) < 2 {
		handleError(conn, "Usage: AUTH username password")
		return
	}

	username := args[0]
	password := args[1]

	creds := loadCredentials()

	user, exists := creds[username]
	if !exists {
		conn.Write([]byte("401 Unauthorized\n"))
		return
	}

	// Verify password with bcrypt
	err := bcrypt.CompareHashAndPassword([]byte(user.Hash), []byte(password))
	if err != nil {
		conn.Write([]byte("401 Unauthorized\n"))
		return
	}

	// Add to UUID → username map
	uuidsMap[user.UUID] = username

	conn.Write([]byte("200 OK\n" + user.UUID + "\n"))
}

// ======= GET =======
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

// ======= DELETE =======
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

// ======= ERROR =======
func handleError(conn net.Conn, msg string) {
	conn.Write([]byte("400 ERROR\n" + msg + "\n"))
}

// ======= UTIL =======

// Generate a simple UUID (replace with proper UUID for production)
func generateUUID(username, password string) string {
	return fmt.Sprintf("%x", username+password)
}

// Load credentials from JSON file
func loadCredentials() map[string]Credentials {
	creds := map[string]Credentials{}
	path := credentialsPath + "/" + credentialsFile
	if _, err := os.Stat(path); err == nil {
		data, err := os.ReadFile(path)
		if err == nil {
			json.Unmarshal(data, &creds)
		}
	}
	return creds
}

// Save credentials to JSON file
func saveCredentials(creds map[string]Credentials) {
	data, _ := json.MarshalIndent(creds, "", "  ")
	os.WriteFile(credentialsPath+"/"+credentialsFile, data, 0600)
}

// Load UUID → username map from credentials at startup
func loadUUIDs() {
	creds := loadCredentials()
	for _, user := range creds {
		uuidsMap[user.UUID] = user.Username
	}
}
