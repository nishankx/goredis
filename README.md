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
All commands are routed through an extensible handler dispatch map (`Handlers`), supporting 25 core Redis commands across 5 functional categories:

#### Connection Commands
| Command | Syntax | Description |
|---|---|---|
| **PING** | `PING [message]` | Tests connection liveness. Returns `PONG` or echoes optional message. |
| **ECHO** | `ECHO message` | Echoes the provided message argument. |

#### String Commands
| Command | Syntax | Description |
|---|---|---|
| **SET** | `SET key value` | Stores a key-value string pair in memory (clears any existing hash at key). Returns `OK`. |
| **GET** | `GET key` | Retrieves the string value for a key, `(nil)` if not found, or `WRONGTYPE` error if key holds a hash. |
| **MSET** | `MSET key value [key value ...]` | Sets multiple key-value pairs in a single operation. Returns `OK`. |
| **MGET** | `MGET key [key ...]` | Returns the values of all specified keys. Returns `(nil)` for missing or non-string keys. |
| **SETNX** | `SETNX key value` | Sets key to hold string value if key does not exist. Returns `1` if key was set, `0` if key already exists. |
| **APPEND** | `APPEND key value` | Appends value to existing string or creates key if it does not exist. Returns integer length of string. |
| **STRLEN** | `STRLEN key` | Returns the integer character length of string stored at key, or `0` if key does not exist. |

#### Numeric & Atomic Counter Commands
| Command | Syntax | Description |
|---|---|---|
| **INCR** | `INCR key` | Increments the integer value of a key by 1. Returns incremented value. |
| **DECR** | `DECR key` | Decrements the integer value of a key by 1. Returns decremented value. |
| **INCRBY** | `INCRBY key increment` | Increments key by specified 64-bit integer amount. Detects arithmetic overflow. |
| **DECRBY** | `DECRBY key decrement` | Decrements key by specified 64-bit integer amount. Detects arithmetic overflow. |

#### Key Management Commands
| Command | Syntax | Description |
|---|---|---|
| **DEL** | `DEL key [key ...]` | Removes specified keys from string and hash stores. Returns integer count of removed keys. |
| **EXISTS** | `EXISTS key [key ...]` | Checks existence of specified keys. Returns integer count of existing keys. |
| **TYPE** | `TYPE key` | Returns simple string data type of key: `string`, `hash`, or `none`. |
| **RENAME** | `RENAME key newkey` | Renames `key` to `newkey`, replacing existing target. Returns `OK` or `ERR no such key`. |
| **DBSIZE** | `DBSIZE` | Returns total integer count of unique keys across all stores. |
| **KEYS** | `KEYS pattern` | Returns matching keys as a sorted array. Supports `*` wildcard or exact key match. |

#### Hash Commands
| Command | Syntax | Description |
|---|---|---|
| **HSET** | `HSET hash field value` | Sets field in a hash map. Returns `1` if field is newly created, `0` if field was updated. |
| **HGET** | `HGET hash field` | Gets a field value from a hash map, or `(nil)`. |
| **HGETALL** | `HGETALL hash` | Returns all field-value pairs from a hash map as an array (alphabetically sorted). |
| **HMSET** | `HMSET hash field value [field value ...]` | Sets multiple field-value pairs within a hash simultaneously. Returns `OK`. |
| **HMGET** | `HMGET hash field [field ...]` | Returns the values associated with the specified fields in a hash. |
| **HDEL** | `HDEL hash field [field ...]` | Removes one or more fields from a hash. Deletes hash when empty. Returns count of removed fields. |
| **HEXISTS** | `HEXISTS hash field` | Checks if field exists within a hash map. Returns `1` if exists, `0` if not. |
| **HLEN** | `HLEN hash` | Returns the integer count of fields contained within a hash map. |
| **HKEYS** | `HKEYS hash` | Returns a sorted array containing all field names within a hash map. |
| **HVALS** | `HVALS hash` | Returns an array of all values within a hash map (sorted by corresponding field name). |

### 3. Concurrency & In-Memory Storage (`handler.go`)
* **String Store (`SETs`)**: Backed by `map[string]string` guarded by `sync.RWMutex`.
* **Hash Store (`HSETs`)**: Backed by `map[string]map[string]string` guarded by `sync.RWMutex`.
* **Lock Ordering & Concurrency Safety**: Strict lock acquisition ordering (`SETsMu` followed by `HSETsMu`) prevents deadlocks during cross-store operations (such as `RENAME`, `DEL`, `EXISTS`, `DBSIZE`, `KEYS`, and type conversions).
* **Type Validation**: Enforces strict Redis type checking, returning `WRONGTYPE Operation against a key holding the wrong kind of value` when string operations are attempted on hash keys or vice versa.

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
├── handler.go    # In-memory storage & command routing logic (25 commands)
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

#### Connection & Strings
```text
127.0.0.1:6380> PING
PONG
127.0.0.1:6380> ECHO "Hello GoRedis"
"Hello GoRedis"
127.0.0.1:6380> SET user:1 "Alice"
OK
127.0.0.1:6380> GET user:1
"Alice"
127.0.0.1:6380> APPEND user:1 " Smith"
(integer) 11
127.0.0.1:6380> STRLEN user:1
(integer) 11
127.0.0.1:6380> MSET k1 "val1" k2 "val2"
OK
127.0.0.1:6380> MGET k1 k2 missing
1) "val1"
2) "val2"
3) (nil)
```

#### Numeric Counters
```text
127.0.0.1:6380> SET page:views "100"
OK
127.0.0.1:6380> INCR page:views
(integer) 101
127.0.0.1:6380> INCRBY page:views 25
(integer) 126
127.0.0.1:6380> DECR page:views
(integer) 125
127.0.0.1:6380> DECRBY page:views 50
(integer) 75
```

#### Key Management
```text
127.0.0.1:6380> EXISTS user:1 page:views missing
(integer) 2
127.0.0.1:6380> TYPE user:1
string
127.0.0.1:6380> RENAME user:1 user:admin
OK
127.0.0.1:6380> DBSIZE
(integer) 4
127.0.0.1:6380> KEYS *
1) "k1"
2) "k2"
3) "page:views"
4) "user:admin"
127.0.0.1:6380> DEL k1 k2
(integer) 2
```

#### Hashes
```text
127.0.0.1:6380> HSET user:profile name "Alice"
(integer) 1
127.0.0.1:6380> HMSET user:profile role "Engineer" location "SF"
OK
127.0.0.1:6380> HGET user:profile role
"Engineer"
127.0.0.1:6380> HMGET user:profile name location
1) "Alice"
2) "SF"
127.0.0.1:6380> HEXISTS user:profile role
(integer) 1
127.0.0.1:6380> HLEN user:profile
(integer) 3
127.0.0.1:6380> HKEYS user:profile
1) "location"
2) "name"
3) "role"
127.0.0.1:6380> HVALS user:profile
1) "SF"
2) "Alice"
3) "Engineer"
127.0.0.1:6380> HGETALL user:profile
1) "location"
2) "SF"
3) "name"
4) "Alice"
5) "role"
6) "Engineer"
127.0.0.1:6380> HDEL user:profile location
(integer) 1
```

---

## Roadmap & Milestones

The following milestones are planned for future releases:

### Milestone 1: Multi-Client Concurrency (High Priority)
- [ ] **Concurrent Connections**: Move connection handling inside `main.go` into a `go handleConnection(conn)` goroutine so multiple clients can interact simultaneously.
- [ ] **Connection Keep-Alive & EOF Handling**: Gracefully handle client disconnects without terminating the server loop.

### Milestone 2: Core Command Expansion
- [x] **Key Management**: Implemented `DEL`, `EXISTS`, `TYPE`, `RENAME`, `DBSIZE`, and `KEYS`.
- [x] **Atomic Counters**: Implemented `INCR`, `DECR`, `INCRBY`, `DECRBY` with 64-bit integer overflow protection.
- [x] **Extended Strings & Hashes**: Implemented `MSET`, `MGET`, `SETNX`, `APPEND`, `STRLEN`, `HMSET`, `HMGET`, `HDEL`, `HEXISTS`, `HLEN`, `HKEYS`, `HVALS`.
- [ ] **Expiration Support**: Implement `EXPIRE`, `PEXPIRE`, `TTL`, `PTTL` with active/passive expiration cleaner.
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
