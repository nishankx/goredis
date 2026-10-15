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

		_ = value
		writer := NewWriter(conn)
		handler, ok := Handlers[command]
		
		if !ok {
			fmt.Println("Invalid command: ", command)
			writer.Write(Value{typ: "string", str: ""})
			continue
		}

		result := handler(args)
		writer.Write(result)
	}
    
}