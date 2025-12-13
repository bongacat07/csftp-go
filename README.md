# File Transfer Protocol (Sort Of)

A bare-bones file transfer protocol built from scratch in Go. No frameworks, no security, no production-readiness. Just raw TCP and ignoring best practices.

## What It Does

Implements three operations over TCP:
- `PUT` - Upload a file to the server
- `GET` - Download a file from the server  
- `DELETE` - Delete a file from the server

That's it. That's the whole protocol.

## Why This Exists

To understand what protocols actually are by building one without any safety rails. Turns out they're not black magic - just a parser, some handler functions, and TCP doing the heavy lifting.

## How It Works

**Protocol Format:**
```
[VERB] [filename]\n
[optional: raw file bytes until connection closes]
```

**Examples:**
- `PUT myfile.txt` followed by file contents
- `GET myfile.txt` 
- `DELETE myfile.txt`

The server listens on port 8080, parses the first line, and routes to the appropriate handler.

## Running It

```bash
go run main.go
```

Server starts on `:8080`. Connect with any TCP client (netcat, telnet, custom client).

**Example with netcat:**
```bash
# Upload a file
cat myfile.txt | nc localhost 8080 <<< "PUT myfile.txt"

# Download a file
echo "GET myfile.txt" | nc localhost 8080 > output.txt

# Delete a file
echo "DELETE myfile.txt" | nc localhost 8080
```

## Security Issues (Yes, All of Them)

- ❌ **No authentication** - Anyone can connect and do anything
- ❌ **Path traversal** - `GET ../../../etc/passwd` works perfectly
- ❌ **No input validation** - Filenames are used directly
- ❌ **Resource exhaustion** - Unlimited concurrent connections
- ❌ **No encryption** - Everything in plaintext
- ❌ **No length framing** - Request parsing can fail if packets split weird
- ❌ **No error recovery** - Failed transfers leave corrupted partial files
- ❌ **No rate limiting** - Can be DOSed by a 5 yr old
- ❌ **No protocol versioning** - Can't evolve without breaking everything

## What I Learned

Building this broken thing taught me why all those security features exist - not from reading about them, but from feeling the pain of not having them.

## What's Next

This is v0.1 - the "make it work" version. Next steps:

### Planned Improvements
- **Authentication & Authorization** - Token-based auth, per-user file isolation
- **Input Validation** - Sanitize filenames, prevent path traversal
- **Protocol Framing** - Length-prefixed messages for reliable parsing
- **Error Recovery** - Checksums, resume failed transfers
- **Rate Limiting** - Per-IP connection and bandwidth limits
- **Encryption** - TLS support

### The Nuclear Option
- **DPDK Kernel Bypass** - Because why settle for regular fast when you can skip the kernel entirely and touch raw NICs? Planning to implement zero-copy transfers with DPDK for maximum throughput.

## Should You Use This?

**Right now?** God no. This is a learning exercise, not software.

**Eventually?** Maybe. Check back when the security holes are patched and DPDK is working.

But should you build your own broken version of something to understand it? Absolutely.

## License

Do whatever you want with this. It's already insecure, what's the worst that could happen?

---

*Built to learn. Broken by design.*

*For now.*
