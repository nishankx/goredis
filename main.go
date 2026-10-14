package main

import (
	"fmt"
	"net"
)

func main() {


	listener, err := net.Listen("tcp", ":6379")
	if err != nil {
		fmt.Println("Error starting server:", err)
		return
	}
	fmt.Println("Server running on :6379")



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

		fmt.Println(value)
		response := "+OK\r\n"
		conn.Write([]byte(response))
	}
    
}