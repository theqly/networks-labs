package main

import (
	"fmt"
	"net"
)

func handshake(conn net.Conn, address string) bool {

	handshake_message := []byte{0x55, 0x01, 0x00}
	_, err := conn.Write(handshake_message)
	if err != nil {
		fmt.Println("error with writing to: ", address)
		return true
	}

	response := make([]byte, 2)
	_, err = conn.Read(response)
	if err != nil {
		fmt.Println("error with reading from: ", address)
		return true
	}

	if response[0] == 0x55 && response[1] == 0x00 {
		fmt.Println("OK")
	} else {
		fmt.Println("ERROR")
	}

	return false
}

func main() {
	address := "localhost:12345"

	conn, err := net.Dial("tcp", address)
	if err != nil {
		fmt.Println("error with connecting to: ", address)
		return
	}
	defer conn.Close()

	handshake(conn, address)
}
