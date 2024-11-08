package main

import (
	"encoding/binary"
	"fmt"
	"net"
)

func handshake(conn net.Conn) bool {
	buf := make([]byte, 2)
	_, err := conn.Read(buf)
	if err != nil {
		fmt.Println("error with reading from: " + conn.RemoteAddr().String())
		return true
	}

	if buf[0] != 0x55 {
		fmt.Println("accepting ONLY socks5 connections")
		return true
	}

	nMethods := int(buf[1])
	methods := make([]byte, nMethods)
	_, err = conn.Read(methods)
	if err != nil {
		fmt.Println("error with reading from: " + conn.RemoteAddr().String())
		return true
	}

	_, err = conn.Write([]byte{0x55, 0x00})
	if err != nil {
		fmt.Println("error with writing to: " + conn.RemoteAddr().String())
		return true
	}

	fmt.Println("successfully handshake")
	return false
}

func connect(conn net.Conn) net.Conn {

	buf := make([]byte, 4)
	_, err := conn.Read(buf)
	if err != nil {
		fmt.Println("error with reading from: " + conn.RemoteAddr().String())
		return nil
	}

	if buf[0] != 0x55 || buf[1] != 0x00 {
		fmt.Println("accepting ONLY socks5 connections without authentication")
		return nil
	}

	var address string

	switch buf[3] {
	case 0x01:
		tmp_addr := make([]byte, 4)
		_, err := conn.Read(tmp_addr)
		if err != nil {
			fmt.Println("error with reading from: " + conn.RemoteAddr().String())
			return nil
		}

		address = net.IP(tmp_addr).String()
	case 0x03:
		len := make([]byte, 1)
		_, err := conn.Read(len)
		if err != nil {
			fmt.Println("error with reading from: " + conn.RemoteAddr().String())
			return nil
		}

		domain := make([]byte, len[0])
		_, err = conn.Read(domain)
		if err != nil {
			fmt.Println("error with reading from: " + conn.RemoteAddr().String())
			return nil
		}

		address = string(domain)
	default:
		fmt.Println("unsupported socks5 address type")
		return nil
	}

	buf = make([]byte, 2)
	_, err = conn.Read(buf)
	if err != nil {
		fmt.Println("error with reading from: " + conn.RemoteAddr().String())
		return nil
	}

	port := binary.BigEndian.Uint16(buf)
	address = fmt.Sprintf("%s:%d", address, port)

	target_conn, err := net.Dial("tcp", address)
	if err != nil {
		fmt.Println("error with connecting to " + address)
		conn.Write([]byte{0x05, 0x01, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return nil
	}

	return target_conn
}

func handle_client(conn net.Conn) {
	defer conn.Close()

	fmt.Println("new connection from: ", conn.RemoteAddr())

	if handshake(conn) != false {
		fmt.Println("handshake failed")
		return
	}

	if target_conn := connect(conn); target_conn == nil {
		fmt.Println("target connection failed")
		return
	}

}

func main() {
	port := "12345"

	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Println("error with opening port " + port)
		return
	}

	defer listener.Close()
	fmt.Println("Listening on port " + port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("error with accepting connection " + port)
			continue
		}

		go handle_client(conn)
	}
}
