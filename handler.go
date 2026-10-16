
package main

import (
	"sort"
	"strconv"
	"sync"
)

// Handlers maps Redis command names to their implementations.
var Handlers = map[string]func([]Value) Value{
	// Connection
	"PING": ping,
	"ECHO": echo,

	// Strings
	"SET":    set,
	"GET":    get,
	"MSET":   mset,
	"MGET":   mget,
	"SETNX":  setnx,
	"APPEND": appendValue,
	"STRLEN": strlen,
	"INCR":   incr,
	"DECR":   decr,
	"INCRBY": incrby,
	"DECRBY": decrby,

	// Keys
	"DEL":    del,
	"EXISTS": exists,
	"TYPE":   typeOf,
	"RENAME": renameKey,
	"DBSIZE": dbsize,
	"KEYS":   keys,

	// Hashes
	"HSET":    hset,
	"HGET":    hget,
	"HGETALL": hgetall,
	"HMSET":   hmset,
	"HMGET":   hmget,
	"HDEL":    hdel,
	"HEXISTS": hexists,
	"HLEN":    hlen,
	"HKEYS":   hkeys,
	"HVALS":   hvals,
}

// -----------------------------------------------------------------------------
// Storage
// -----------------------------------------------------------------------------

var SETs = map[string]string{}
var SETsMu sync.RWMutex

var HSETs = map[string]map[string]string{}
var HSETsMu sync.RWMutex

func errorValue(message string) Value {
	return Value{typ: "error", str: message}
}

func integerValue(n int64) Value {
	return Value{typ: "integer", str: strconv.FormatInt(n, 10)}
}

func bulkValue(s string) Value {
	return Value{typ: "bulk", bulk: s}
}

func arrayValue(values []Value) Value {
	return Value{typ: "array", array: values}
}

func wrongArgs(command string) Value {
	return errorValue(
		"ERR wrong number of arguments for '" + command + "' command",
	)
}

// -----------------------------------------------------------------------------
// Connection commands
// -----------------------------------------------------------------------------

func ping(args []Value) Value {
	if len(args) == 0 {
		return Value{typ: "string", str: "PONG"}
	}

	if len(args) != 1 {
		return wrongArgs("ping")
	}

	return bulkValue(args[0].bulk)
}

func echo(args []Value) Value {
	if len(args) != 1 {
		return wrongArgs("echo")
	}

	return bulkValue(args[0].bulk)
}

// -----------------------------------------------------------------------------
// String commands
// -----------------------------------------------------------------------------

func set(args []Value) Value {
	if len(args) != 2 {
		return wrongArgs("set")
	}

	key := args[0].bulk
	value := args[1].bulk

	// Keep both locks in a consistent order whenever both are needed.
	SETsMu.Lock()
	HSETsMu.Lock()

	SETs[key] = value
	delete(HSETs, key)

	HSETsMu.Unlock()
	SETsMu.Unlock()

	return Value{typ: "string", str: "OK"}
}

func get(args []Value) Value {
	if len(args) != 1 {
		return wrongArgs("get")
	}

	key := args[0].bulk

	SETsMu.RLock()
	value, ok := SETs[key]
	SETsMu.RUnlock()

	if !ok {
		HSETsMu.RLock()
		_, isHash := HSETs[key]
		HSETsMu.RUnlock()

		if isHash {
			return errorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
		}

		return Value{typ: "null"}
	}

	return bulkValue(value)
}

func mset(args []Value) Value {
	if len(args) < 2 || len(args)%2 != 0 {
		return wrongArgs("mset")
	}

	SETsMu.Lock()
	HSETsMu.Lock()

	for i := 0; i < len(args); i += 2 {
		key := args[i].bulk
		value := args[i+1].bulk

		SETs[key] = value
		delete(HSETs, key)
	}

	HSETsMu.Unlock()
	SETsMu.Unlock()

	return Value{typ: "string", str: "OK"}
}

func mget(args []Value) Value {
	if len(args) == 0 {
		return wrongArgs("mget")
	}

	SETsMu.RLock()
	HSETsMu.RLock()

	result := make([]Value, 0, len(args))

	for _, arg := range args {
		key := arg.bulk

		if value, ok := SETs[key]; ok {
			result = append(result, bulkValue(value))
		} else if _, isHash := HSETs[key]; isHash {
			// Redis MGET returns null for keys holding non-string values.
			result = append(result, Value{typ: "null"})
		} else {
			result = append(result, Value{typ: "null"})
		}
	}

	HSETsMu.RUnlock()
	SETsMu.RUnlock()

	return arrayValue(result)
}

func setnx(args []Value) Value {
	if len(args) != 2 {
		return wrongArgs("setnx")
	}

	key := args[0].bulk

	SETsMu.Lock()
	HSETsMu.Lock()

	defer SETsMu.Unlock()
	defer HSETsMu.Unlock()

	if _, exists := SETs[key]; exists {
		return integerValue(0)
	}

	if _, exists := HSETs[key]; exists {
		return integerValue(0)
	}

	SETs[key] = args[1].bulk

	return integerValue(1)
}

func appendValue(args []Value) Value {
	if len(args) != 2 {
		return wrongArgs("append")
	}

	key := args[0].bulk
	suffix := args[1].bulk

	SETsMu.Lock()
	HSETsMu.Lock()

	defer SETsMu.Unlock()
	defer HSETsMu.Unlock()

	if _, isHash := HSETs[key]; isHash {
		return errorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	SETs[key] += suffix

	return integerValue(int64(len(SETs[key])))
}

func strlen(args []Value) Value {
	if len(args) != 1 {
		return wrongArgs("strlen")
	}

	key := args[0].bulk

	SETsMu.RLock()
	value, exists := SETs[key]
	SETsMu.RUnlock()

	if !exists {
		HSETsMu.RLock()
		_, isHash := HSETs[key]
		HSETsMu.RUnlock()

		if isHash {
			return errorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
		}

		return integerValue(0)
	}

	return integerValue(int64(len(value)))
}

// -----------------------------------------------------------------------------
// Numeric commands
// -----------------------------------------------------------------------------

func incr(args []Value) Value {
	if len(args) != 1 {
		return wrongArgs("incr")
	}

	return incrementBy(args[0].bulk, 1)
}

func decr(args []Value) Value {
	if len(args) != 1 {
		return wrongArgs("decr")
	}

	return incrementBy(args[0].bulk, -1)
}

func incrby(args []Value) Value {
	if len(args) != 2 {
		return wrongArgs("incrby")
	}

	amount, err := strconv.ParseInt(args[1].bulk, 10, 64)
	if err != nil {
		return errorValue("ERR value is not an integer or out of range")
	}

	return incrementBy(args[0].bulk, amount)
}

func decrby(args []Value) Value {
	if len(args) != 2 {
		return wrongArgs("decrby")
	}

	amount, err := strconv.ParseInt(args[1].bulk, 10, 64)
	if err != nil {
		return errorValue("ERR value is not an integer or out of range")
	}

	if amount == -1<<63 {
		return errorValue("ERR increment or decrement would overflow")
	}

	return incrementBy(args[0].bulk, -amount)
}

func incrementBy(key string, amount int64) Value {
	SETsMu.Lock()
	HSETsMu.Lock()

	defer SETsMu.Unlock()
	defer HSETsMu.Unlock()

	if _, isHash := HSETs[key]; isHash {
		return errorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	var current int64

	if value, exists := SETs[key]; exists {
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return errorValue("ERR value is not an integer or out of range")
		}

		current = parsed
	}

	// Detect overflow before adding.
	const maxInt64 = int64(1<<63 - 1)
	const minInt64 = -maxInt64 - 1

	if (amount > 0 && current > maxInt64-amount) ||
		(amount < 0 && current < minInt64-amount) {
		return errorValue("ERR increment or decrement would overflow")
	}

	current += amount
	SETs[key] = strconv.FormatInt(current, 10)

	return integerValue(current)
}

// -----------------------------------------------------------------------------
// Key management commands
// -----------------------------------------------------------------------------

func del(args []Value) Value {
	if len(args) == 0 {
		return wrongArgs("del")
	}

	SETsMu.Lock()
	HSETsMu.Lock()

	defer SETsMu.Unlock()
	defer HSETsMu.Unlock()

	var deleted int64

	for _, arg := range args {
		key := arg.bulk

		_, hasString := SETs[key]
		_, hasHash := HSETs[key]

		if hasString || hasHash {
			delete(SETs, key)
			delete(HSETs, key)
			deleted++
		}
	}

	return integerValue(deleted)
}

func exists(args []Value) Value {
	if len(args) == 0 {
		return wrongArgs("exists")
	}

	SETsMu.RLock()
	HSETsMu.RLock()

	defer SETsMu.RUnlock()
	defer HSETsMu.RUnlock()

	var count int64

	for _, arg := range args {
		key := arg.bulk

		if _, ok := SETs[key]; ok {
			count++
		} else if _, ok := HSETs[key]; ok {
			count++
		}
	}

	return integerValue(count)
}

func typeOf(args []Value) Value {
	if len(args) != 1 {
		return wrongArgs("type")
	}

	key := args[0].bulk

	SETsMu.RLock()
	_, hasString := SETs[key]
	SETsMu.RUnlock()

	HSETsMu.RLock()
	_, hasHash := HSETs[key]
	HSETsMu.RUnlock()

	if hasString {
		return Value{typ: "string", str: "string"}
	}

	if hasHash {
		return Value{typ: "string", str: "hash"}
	}

	return Value{typ: "string", str: "none"}
}

func renameKey(args []Value) Value {
	if len(args) != 2 {
		return wrongArgs("rename")
	}

	oldKey := args[0].bulk
	newKey := args[1].bulk

	SETsMu.Lock()
	HSETsMu.Lock()

	defer SETsMu.Unlock()
	defer HSETsMu.Unlock()

	if oldKey == newKey {
		if _, ok := SETs[oldKey]; ok {
			return Value{typ: "string", str: "OK"}
		}

		if _, ok := HSETs[oldKey]; ok {
			return Value{typ: "string", str: "OK"}
		}

		return errorValue("ERR no such key")
	}

	if value, ok := SETs[oldKey]; ok {
		SETs[newKey] = value
		delete(HSETs, newKey)
		delete(SETs, oldKey)

		return Value{typ: "string", str: "OK"}
	}

	if value, ok := HSETs[oldKey]; ok {
		HSETs[newKey] = value
		delete(SETs, newKey)
		delete(HSETs, oldKey)

		return Value{typ: "string", str: "OK"}
	}

	return errorValue("ERR no such key")
}

func dbsize(args []Value) Value {
	if len(args) != 0 {
		return wrongArgs("dbsize")
	}

	SETsMu.RLock()
	HSETsMu.RLock()

	defer SETsMu.RUnlock()
	defer HSETsMu.RUnlock()

	count := len(SETs)

	for key := range HSETs {
		if _, exists := SETs[key]; !exists {
			count++
		}
	}

	return integerValue(int64(count))
}

// KEYS currently supports "*" to return every key.
// Other patterns are treated as exact key matches.
func keys(args []Value) Value {
	if len(args) != 1 {
		return wrongArgs("keys")
	}

	pattern := args[0].bulk

	SETsMu.RLock()
	HSETsMu.RLock()

	allKeys := make(map[string]struct{}, len(SETs)+len(HSETs))

	for key := range SETs {
		allKeys[key] = struct{}{}
	}

	for key := range HSETs {
		allKeys[key] = struct{}{}
	}

	HSETsMu.RUnlock()
	SETsMu.RUnlock()

	matched := make([]string, 0)

	for key := range allKeys {
		if pattern == "*" || key == pattern {
			matched = append(matched, key)
		}
	}

	sort.Strings(matched)

	result := make([]Value, 0, len(matched))

	for _, key := range matched {
		result = append(result, bulkValue(key))
	}

	return arrayValue(result)
}

// -----------------------------------------------------------------------------
// Hash commands
// -----------------------------------------------------------------------------

func hset(args []Value) Value {
	if len(args) != 3 {
		return wrongArgs("hset")
	}

	hash := args[0].bulk
	field := args[1].bulk
	value := args[2].bulk

	SETsMu.Lock()
	HSETsMu.Lock()

	defer SETsMu.Unlock()
	defer HSETsMu.Unlock()

	if _, isString := SETs[hash]; isString {
		return errorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	if _, exists := HSETs[hash]; !exists {
		HSETs[hash] = make(map[string]string)
	}

	_, fieldExists := HSETs[hash][field]
	HSETs[hash][field] = value

	if fieldExists {
		return integerValue(0)
	}

	return integerValue(1)
}

func hget(args []Value) Value {
	if len(args) != 2 {
		return wrongArgs("hget")
	}

	hash := args[0].bulk
	field := args[1].bulk

	SETsMu.RLock()
	_, isString := SETs[hash]
	SETsMu.RUnlock()

	if isString {
		return errorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	HSETsMu.RLock()
	value, exists := HSETs[hash][field]
	HSETsMu.RUnlock()

	if !exists {
		return Value{typ: "null"}
	}

	return bulkValue(value)
}

func hgetall(args []Value) Value {
	if len(args) != 1 {
		return wrongArgs("hgetall")
	}

	hash := args[0].bulk

	SETsMu.RLock()
	_, isString := SETs[hash]
	SETsMu.RUnlock()

	if isString {
		return errorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	HSETsMu.RLock()
	fields := HSETs[hash]

	// Copy the map while locked so we can sort it after unlocking.
	copied := make(map[string]string, len(fields))
	for field, value := range fields {
		copied[field] = value
	}
	HSETsMu.RUnlock()

	fieldNames := make([]string, 0, len(copied))
	for field := range copied {
		fieldNames = append(fieldNames, field)
	}

	sort.Strings(fieldNames)

	result := make([]Value, 0, len(fieldNames)*2)

	for _, field := range fieldNames {
		result = append(result, bulkValue(field))
		result = append(result, bulkValue(copied[field]))
	}

	return arrayValue(result)
}

func hmset(args []Value) Value {
	if len(args) < 3 || len(args)%2 == 0 {
		return wrongArgs("hmset")
	}

	hash := args[0].bulk

	SETsMu.Lock()
	HSETsMu.Lock()

	defer SETsMu.Unlock()
	defer HSETsMu.Unlock()

	if _, isString := SETs[hash]; isString {
		return errorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	if _, exists := HSETs[hash]; !exists {
		HSETs[hash] = make(map[string]string)
	}

	for i := 1; i < len(args); i += 2 {
		HSETs[hash][args[i].bulk] = args[i+1].bulk
	}

	return Value{typ: "string", str: "OK"}
}

func hmget(args []Value) Value {
	if len(args) < 2 {
		return wrongArgs("hmget")
	}

	hash := args[0].bulk

	SETsMu.RLock()
	_, isString := SETs[hash]
	SETsMu.RUnlock()

	if isString {
		return errorValue("WRONGTYPE Operation against a key holding the wrong kind of value")
	}

	HSETsMu.RLock()
	defer HSETsMu.RUnlock()

	result := make([]Value, 0, len(args)-1)

	for _, arg := range args[1:] {
		value, exists := HSETs[hash][arg.bulk]

		if !exists {
			result = append(result, Value{typ: "null"})
			continue
		}

		result = append(result, bulkValue(value))
	}

	return arrayValue(result)
}

func hdel(args []Value) Value {
	if len(args) < 2 {
		return wrongArgs("hdel")
	}

	hash := args[0].bulk

	HSETsMu.Lock()
	defer HSETsMu.Unlock()

	fields, exists := HSETs[hash]
	if !exists {
		return integerValue(0)
	}

	var deleted int64

	for _, arg := range args[1:] {
		if _, exists := fields[arg.bulk]; exists {
			delete(fields, arg.bulk)
			deleted++
		}
	}

	if len(fields) == 0 {
		delete(HSETs, hash)
	}

	return integerValue(deleted)
}

func hexists(args []Value) Value {
	if len(args) != 2 {
		return wrongArgs("hexists")
	}

	hash := args[0].bulk
	field := args[1].bulk

	HSETsMu.RLock()
	_, exists := HSETs[hash][field]
	HSETsMu.RUnlock()

	if exists {
		return integerValue(1)
	}

	return integerValue(0)
}

func hlen(args []Value) Value {
	if len(args) != 1 {
		return wrongArgs("hlen")
	}

	HSETsMu.RLock()
	length := len(HSETs[args[0].bulk])
	HSETsMu.RUnlock()

	return integerValue(int64(length))
}

func hkeys(args []Value) Value {
	if len(args) != 1 {
		return wrongArgs("hkeys")
	}

	HSETsMu.RLock()

	fields := HSETs[args[0].bulk]
	names := make([]string, 0, len(fields))

	for field := range fields {
		names = append(names, field)
	}

	HSETsMu.RUnlock()

	sort.Strings(names)

	result := make([]Value, 0, len(names))

	for _, field := range names {
		result = append(result, bulkValue(field))
	}

	return arrayValue(result)
}

func hvals(args []Value) Value {
	if len(args) != 1 {
		return wrongArgs("hvals")
	}

	HSETsMu.RLock()

	fields := HSETs[args[0].bulk]
	names := make([]string, 0, len(fields))

	for field := range fields {
		names = append(names, field)
	}

	sort.Strings(names)

	result := make([]Value, 0, len(names))

	for _, field := range names {
		result = append(result, bulkValue(fields[field]))
	}

	HSETsMu.RUnlock()

	return arrayValue(result)
}
