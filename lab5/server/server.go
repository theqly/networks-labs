package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
)

func handshake(conn net.Conn) bool {
	buf := make([]byte, 2)
	_, err := conn.Read(buf)
	if err != nil {
		log.Printf("Error reading from %s: %v", conn.RemoteAddr().String(), err)
		return true
	}

	if buf[0] != 0x05 {
		log.Printf("Accepting ONLY SOCKS5 connections, got: %x", buf[0])
		return true
	}

	nMethods := int(buf[1])
	methods := make([]byte, nMethods)
	_, err = conn.Read(methods)
	if err != nil {
		log.Printf("Error reading from %s: %v", conn.RemoteAddr().String(), err)
		return true
	}

	_, err = conn.Write([]byte{0x05, 0x00})
	if err != nil {
		log.Printf("Error writing to %s: %v", conn.RemoteAddr().String(), err)
		return true
	}

	log.Printf("Handshake successful with client %s", conn.RemoteAddr().String())
	return false
}

func connect(conn net.Conn) net.Conn {
	buf := make([]byte, 4)
	_, err := conn.Read(buf)
	if err != nil {
		connected_send(conn, 0x01)
		log.Printf("Error reading from %s: %v", conn.RemoteAddr().String(), err)
		return nil
	}

	if buf[0] != 0x05 {
		connected_send(conn, 0x07)
		log.Printf("Accepting ONLY SOCKS5 connections, got: %x", buf[0])
		return nil
	}

	if buf[1] != 0x01 {
		connected_send(conn, 0x07)
		log.Printf("Unknown command: %x", buf[1])
		return nil
	}

	var address string
	switch buf[3] {
	case 0x01:
		tmpAddr := make([]byte, 4)
		_, err := conn.Read(tmpAddr)
		if err != nil {
			connected_send(conn, 0x01)
			log.Printf("Error reading from %s: %v", conn.RemoteAddr().String(), err)
			return nil
		}
		address = net.IP(tmpAddr).String()
	case 0x03:
		lenBuf := make([]byte, 1)
		_, err := conn.Read(lenBuf)
		if err != nil {
			connected_send(conn, 0x01)
			log.Printf("Error reading from %s: %v", conn.RemoteAddr().String(), err)
			return nil
		}
		domain := make([]byte, lenBuf[0])
		_, err = conn.Read(domain)
		if err != nil {
			connected_send(conn, 0x01)
			log.Printf("Error reading from %s: %v", conn.RemoteAddr().String(), err)
			return nil
		}
		address = string(domain)
	default:
		connected_send(conn, 0x08)
		log.Printf("Unsupported SOCKS5 address type: %x", buf[3])
		return nil
	}

	portBuf := make([]byte, 2)
	_, err = conn.Read(portBuf)
	if err != nil {
		connected_send(conn, 0x01)
		log.Printf("Error reading from %s: %v", conn.RemoteAddr().String(), err)
		return nil
	}

	port := binary.BigEndian.Uint16(portBuf)
	address = fmt.Sprintf("%s:%d", address, port)

	targetConn, err := net.Dial("tcp", address)
	if err != nil {
		log.Printf("Error connecting to %s: %v", address, err)
		connected_send(conn, 0x01)
		return nil
	}

	connected_send(conn, 0x00)
	log.Printf("Successfully connected to %s", address)
	return targetConn
}

func connected_send(conn net.Conn, err_code byte) {
	_, err := conn.Write([]byte{0x05, err_code, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	if err != nil {
		log.Printf("Error writing to %s: %v", conn.RemoteAddr().String(), err)
		return
	}
}

func transferData(conn net.Conn, target_conn net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		defer target_conn.(*net.TCPConn).CloseWrite()

		_, err := io.Copy(target_conn, conn)
		if err != nil {
			log.Printf("Error transferring data from %s: %v", conn.RemoteAddr().String(), err)
		}
	}()

	go func() {
		defer wg.Done()
		defer conn.(*net.TCPConn).CloseWrite()

		_, err := io.Copy(conn, target_conn)
		if err != nil {
			log.Printf("Error transferring data to %s: %v", conn.RemoteAddr().String(), err)
		}
	}()

	wg.Wait()
}

func handleClient(conn net.Conn) {
	defer conn.Close()

	log.Printf("New connection from %s", conn.RemoteAddr().String())

	if handshake(conn) {
		log.Println("Handshake failed")
		return
	}

	targetConn := connect(conn)
	if targetConn == nil {
		log.Println("Target connection failed")
		return
	}
	defer targetConn.Close()

	transferData(conn, targetConn)
}

func main() {
	port := "12345"

	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Printf("Error opening port %s: %v", port, err)
		return
	}
	defer listener.Close()
	log.Printf("Listening on port %s", port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Error accepting connection: %v", err)
			continue
		}

		go handleClient(conn)
	}
}
