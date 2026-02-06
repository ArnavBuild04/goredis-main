<p align="center">
  <img src="https://cdn.simpleicons.org/redis/DC382D" alt="GoRedis" width="80" height="80">
</p>

<h1 align="center">GoRedis</h1>

<p align="center">
  <strong>A production-grade Redis server implementation in Go</strong><br>
  <em>Complete with RESP protocol parsing, AOF persistence, and concurrent access</em>
</p>

<p align="center">
  <a href="#architecture">Architecture</a> •
  <a href="#how-it-works">How It Works</a> •
  <a href="#quick-start">Quick Start</a> •
  <a href="#commands">Commands</a> •
  <a href="#roadmap">Roadmap</a>
</p>

---

## ⚡ Features

- **Full RESP Protocol Support** - Parses and marshals all core Redis data types
- **Concurrent-Safe** - Uses `sync.RWMutex` for optimized read-heavy workloads
- **Persistent Storage** - Append-Only File (AOF) with automatic fsync every second
- **Zero Dependencies** - Built entirely with Go's standard library

---

## 🏗️ Architecture

```
                                    ┌────────────────────────────────────────────────────────┐
                                    │                   GoRedis Server                       │
                                    │                                                        │
┌─────────────┐   TCP Connection    │  ┌──────────────┐    ┌─────────────┐    ┌──────────┐  │
│ Redis CLI   │ ──────────────────▶ │  │    main.go   │───▶│  parser.go  │───▶│handler.go│  │
│ (Client)    │                     │  │  (Listener)  │    │   (RESP)    │    │ (Commands │  │
└─────────────┘                     │  └──────┬───────┘    └──────┬──────┘    └─────┬────┘  │
                                    │         │                   │                 │       │
                                    │         │ Accept()          │ Parse           │ R/W   │
                                    │         ▼                   ▼                 ▼       │
                                    │  ┌──────────────┐    ┌─────────────┐    ┌──────────┐  │
                                    │  │  goroutine   │    │    Value    │    │ SETs map │  │
                                    │  │ per client   │    │   struct    │    │HSETs map │  │
                                    │  └──────────────┘    └─────────────┘    └────┬─────┘  │
                                    │                                              │        │
                                    │                                              ▼        │
                                    │                                        ┌──────────┐   │
                                    │                                        │  aof.go  │   │
                                    │                                        │ (db.aof) │   │
                                    │                                        └──────────┘   │
                                    └────────────────────────────────────────────────────────┘

```

| Component       | File          | Responsibility                                           |
|-----------------|---------------|----------------------------------------------------------|
| **TCP Server**  | `main.go`     | Listens on port, spawns goroutines per connection        |
| **RESP Parser** | `parser.go`   | Decodes/encodes Redis Serialization Protocol             |
| **Command Handler** | `handler.go` | Executes commands against in-memory data structures   |
| **Persistence** | `aof.go`      | Append-Only File for durability, auto-fsync per second   |

---

## 🔄 How It Works

The request lifecycle follows a clean pipeline:

```
   ┌─────────────┐      ┌─────────────┐      ┌─────────────┐      ┌─────────────┐
   │   LISTEN    │      │   ACCEPT    │      │    PARSE    │      │   EXECUTE   │
   │  (TCP:6379) │ ───▶ │ (goroutine) │ ───▶ │   (RESP)    │ ───▶ │  (Handler)  │
   └─────────────┘      └─────────────┘      └─────────────┘      └─────────────┘
         │                    │                    │                    │
         ▼                    ▼                    ▼                    ▼
   net.Listen()        net.Accept()         bufio.Reader          map[string]
   Blocks until        Returns net.Conn     Buffered parsing      Lock/Unlock
   client connects     for each client      avoids syscalls       operations
```

### Step-by-Step Breakdown

| Step | Code Location | What Happens |
|------|---------------|--------------|
| **1. Listen** | `main.go:34` | Server binds to `localhost:6379` and blocks on `net.Listen()` |
| **2. Recovery** | `main.go:50-61` | AOF file is read; all `SET`/`HSET` commands are replayed |
| **3. Accept Loop** | `main.go:68-77` | Each client gets a dedicated goroutine via `go r.readLoop()` |
| **4. Read RESP** | `parser.go:110-126` | Byte stream parsed into `Value{typ, array, bulk}` struct |
| **5. Dispatch** | `parser.go:245-265` | Command looked up in `Handlers` map and executed |
| **6. Persist** | `parser.go:261-263` | Write operations (`SET`, `HSET`) appended to `db.aof` |
| **7. Response** | `parser.go:265` | `Value.Marshal()` encodes response → sent to client |

---

## 🚀 Quick Start

### Prerequisites

- Go 1.23.5+

### Build & Run

```bash
# Clone the repository

cd goredis

# Run with default port 6379
go run .

# Or specify a custom port
go run . --port 6380
```

### Connect with redis-cli

```bash
$ redis-cli -p 6379

127.0.0.1:6379> PING
PONG

127.0.0.1:6379> SET mykey "Hello, GoRedis!"
OK

127.0.0.1:6379> GET mykey
"Hello, GoRedis!"

127.0.0.1:6379> HSET user:1 name "Alice" 
(error) Err wrong number of arguments for 'hset' command

127.0.0.1:6379> HSET user:1 name "Alice"
(integer) 1

127.0.0.1:6379> HGETALL user:1
1) "name"
2) "Alice"
```

---

## 📖 Commands

| Command | Syntax | Description | Time Complexity |
|---------|--------|-------------|-----------------|
| `PING`  | `PING [message]` | Test connection; returns `PONG` or echoes message | O(1) |
| `SET`   | `SET key value` | Set key to hold string value | O(1) |
| `GET`   | `GET key` | Get value of key | O(1) |
| `HSET`  | `HSET hash field value` | Set field in hash | O(1) |
| `HGET`  | `HGET hash field` | Get field from hash | O(1) |
| `HGETALL` | `HGETALL hash` | Get all fields/values from hash | O(N) |

---

## 🗂️ Project Structure

```
goredis/
├── main.go          # Server entry point, TCP listener, connection handling
├── parser.go        # RESP protocol parser & writer (state machine)
├── handler.go       # Command implementations with concurrent data structures
├── aof.go           # Append-Only File persistence layer
├── db.aof           # Generated at runtime (your database)
├── go.mod           # Go module definition
└── README.md        # You are here
```

---

## 🛣️ Roadmap

### Implemented ✅
- [x] Core key-value operations (`SET`, `GET`)
- [x] Hash map operations (`HSET`, `HGET`, `HGETALL`)
- [x] RESP protocol parsing
- [x] AOF persistence with auto-sync

### Coming Soon
- [ ] **Data Types**: Lists (`LPUSH`, `LPOP`), Sets (`SADD`, `SMEMBERS`), Sorted Sets
- [ ] **TTL/Expiration**: `EXPIRE`, `TTL`, `SETEX`
- [ ] **Pub/Sub**: `PUBLISH`, `SUBSCRIBE`
- [ ] **Transactions**: `MULTI`, `EXEC`, `WATCH`
- [ ] **Authentication**: `AUTH` command
- [ ] **Graceful Shutdown**: Signal handling
- [ ] **Benchmarking**: Performance test suite

---


<p align="center">
  <strong>Built with ❤️ for learning networking and concurrency in Go</strong>
</p>
