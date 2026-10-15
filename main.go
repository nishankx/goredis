package main

import (
	"fmt"
	"net"
	"strings"
)

func main() {


	listener, err := net.Listen("tcp", ":6380")
	if err != nil {
		fmt.Println("Error starting server:", err)
		return
	}
	fmt.Println("Server running on :6380")

	aof, err := NewAof("database.aof")
	if err != nil {
		fmt.Println("Error starting AOF:", err)
		return
	}
	
	aof.Read(func(value Value) {
		command := strings.ToUpper(value.array[0].bulk)
		args := value.array[1:]
		handler, ok := Handlers[command]
		if !ok {
			fmt.Println("Invalid command: ", command)
			return
		}

		handler(args)
	})
	defer aof.Close()

	conn, err := listener.Accept()
	if err != nil {
		fmt.Println("Error accepting connection:", err)
		return
	}
	defer conn.Close()


	
	for {
		resp := NewResp(conn)
		value, err := resp.Read()
		if err != nil {
			fmt.Println("Error reading from connection:", err)
			return
		}

		if value.typ != "array" {
			fmt.Printf("Invalid request, expected array length > 0")
			continue
		}

		if len(value.array) == 0 {
			fmt.Printf("Invalid request, expected array length > 0")
			continue
		}

		command := strings.ToUpper(value.array[0].bulk)
		args := value.array[1:]
		writer := NewWriter(conn)
		handler, ok := Handlers[command]
		if !ok {
			fmt.Println("Invalid command: ", command)
			continue
		}

		if command == "SET" || command == "HSET"{
			aof.Write(value)
		}

		result := handler(args)
		writer.Write(result)
	}
    
}