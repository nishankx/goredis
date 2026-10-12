package main

import (
	"fmt"
	"io"
	"net"
	"os"
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
		buf := make([]byte, 1024)
		_, err := conn.Read(buf)
		if err != nil {
			if err == io.EOF {
				fmt.Println("Client Disconnected")
				break
			}
			fmt.Println("Error reading from connection:", err)
			os.Exit(1)
		}

		response := "+OK\r\n"
		conn.Write([]byte(response))
	}
    
}