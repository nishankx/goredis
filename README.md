# GoRedis (goredis)

A lightweight, from-scratch implementation of a **Redis-compatible in-memory key-value database server** written in Go.

This project implements the core components of Redis: a custom **RESP (REdis Serialization Protocol)** parser and serializer, an **in-memory data store** with thread-safe locking, a **TCP server**, and **AOF (Append-Only File)** persistence for durability across restarts.

---

## What is GoRedis?

GoRedis is designed to replicate Redis server behavior at the protocol level. Because it adheres to the official RESP protocol specification, standard Redis client libraries and the official `redis-cli` utility can connect and execute commands against GoRedis as if it were a native Redis instance.

---

## Current Implementation

### 1. RESP Protocol Engine (`resp.go`)
Implements full low-level serialization and deserialization for the Redis Serialization Protocol:
* **Parsing / Deserialization (`Resp`)**:
  * Reads raw byte streams using buffered I/O (`bufio.Reader`).
  * Parses arrays (`*`), bulk strings (`$`), and integers (`:`).
  * Safely processes CRLF (`\r\n`) protocol terminators.
* **Serialization / Marshaling (`Value.Marshal`)**:
  * Simple Strings (`+<msg>\r\n`)
  * Errors (`-<err>\r\n`)
  * Bulk Strings (`$<len>\r\n<content>\r\n`)
  * Arrays (`*<count>\r\n...`)
  * Null Bulk Strings (`$-1\r\n`)

### 2. Supported Commands (`handler.go`)
All commands are routed through an extensible handler dispatch map (`Handlers`):

| Command | Syntax | Description |
|---|---|---|
| **PING** | `PING [message]` | Tests connection liveness. Returns `PONG` or echoes optional message. |
| **SET** | `SET key value` | Stores a key-value string pair in memory. Returns `OK`. |
| **GET** | `GET key` | Retrieves the string value for a key, or `(nil)` if not found. |
| **HSET** | `HSET hash key value` | Sets a field within a Redis hash map. Returns `OK`. |
| **HGET** | `HGET hash key` | Gets a field value from a hash map, or `(nil)`. |
| **HGETALL** | `HGETALL hash` | Returns all field-value pairs from a hash map as an array. |

### 3. Concurrency & In-Memory Storage (`handler.go`)
* **String Store (`SETs`)**: Backed by `map[string]string` guarded by `sync.RWMutex` for concurrent read/write safety.
* **Hash Store (`HSETs`)**: Backed by `map[string]map[string]string` guarded by `sync.RWMutex`.

### 4. AOF Persistence (`aof.go`)
* **Write-Ahead Logging**: Every state-modifying command (`SET`, `HSET`) is serialized into RESP and appended to `databse.aof`.
* **Periodic Background Fsync**: A dedicated goroutine runs every 1 second executing `file.Sync()`, balancing write throughput with crash durability.
* **Startup Replay**: Reads and replays previously recorded RESP commands upon server boot to restore state before accepting new connections.

### 5. TCP Networking (`main.go`)
* Binds to TCP port `:6380` (configured to prevent conflicts with default Redis port `6379`).
* Accepts incoming connections and processes requests in an event loop.

---

## Project Structure

```
goredis/
├── aof.go        # Append-Only File persistence engine & background fsync
├── handler.go    # In-memory storage & command routing logic
├── main.go       # TCP server listener, client loop & AOF integration
├── resp.go       # RESP protocol parser, serializer, and Value structures
├── go.mod        # Go module definition
└── README.md     # Project documentation & roadmap
```

---

## Getting Started

### Prerequisites
* [Go 1.18+](https://golang.org/dl/)
* [`redis-cli`](https://redis.io/docs/latest/develop/connect/cli/) (optional, for interacting with the server)

### 1. Build the Binary
```bash
go build -o goredis.exe .
```

### 2. Run the Server
```bash
./goredis.exe
```
Output:
```
Listening on port :6380
Server running on :6380
```

### 3. Connect with `redis-cli`
Open a separate terminal and connect to port `6380`:
```bash
redis-cli -p 6380
```

### 4. Example Usage
```text
127.0.0.1:6380> PING
PONG
127.0.0.1:6380> PING "Hello GoRedis"
"Hello GoRedis"
127.0.0.1:6380> SET user:1 "Alice"
OK
127.0.0.1:6380> GET user:1
"Alice"
127.0.0.1:6380> HSET user:1:profile name "Alice"
OK
127.0.0.1:6380> HSET user:1:profile role "Engineer"
OK
127.0.0.1:6380> HGET user:1:profile role
"Engineer"
127.0.0.1:6380> HGETALL user:1:profile
1) "name"
2) "Alice"
3) "role"
4) "Engineer"
```

---

## Roadmap & Milestones

The following milestones are planned for future releases:

### Milestone 1: Multi-Client Concurrency (High Priority)
- [ ] **Concurrent Connections**: Move connection handling inside `main.go` into a `go handleConnection(conn)` goroutine so multiple clients can interact simultaneously.
- [ ] **Connection Keep-Alive & EOF Handling**: Gracefully handle client disconnects without terminating the server loop.

### Milestone 2: Core Command Expansion
- [ ] **Keys & Expiration**: Implement `DEL`, `EXISTS`, `KEYS`, `EXPIRE`, and `TTL` with a background active/passive expiration cleaner.
- [ ] **Atomic Counters**: Implement `INCR`, `DECR`, `INCRBY`, `DECRBY`.
- [ ] **List Data Type**: Implement `LPUSH`, `RPUSH`, `LPOP`, `RPOP`, `LRANGE`.
- [ ] **Set Data Type**: Implement `SADD`, `SREM`, `SMEMBERS`, `SISMEMBER`.

### Milestone 3: Persistence Enhancements
- [ ] **AOF Log Compaction / Rewrite (`BGREWRITEAOF`)**: Periodically reconstruct the AOF file from current memory state to prevent infinite file growth.
- [ ] **Snapshotting (RDB)**: Add support for point-in-time binary RDB dump and restore.
- [ ] **AOF Replay Fix**: Ensure complete multi-command stream processing during startup recovery.

### Milestone 4: Operational Readiness
- [ ] **CLI Configuration Flags**: Support `--port`, `--host`, `--aof-path`, and `--dir` command-line flags.
- [ ] **Graceful Shutdown**: Trap `SIGINT` / `SIGTERM` signals to cleanly flush buffers, close the AOF handle, and close active connections.
- [ ] **Authentication**: Implement `AUTH` command with password protection.
- [ ] **Unit & Benchmark Testing**: Add comprehensive unit tests for the RESP parser and benchmark read/write throughput using `redis-benchmark`.

---

## License

Open-source and available for educational and commercial use under the [MIT License](LICENSE).
