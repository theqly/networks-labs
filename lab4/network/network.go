package network

import (
	"fmt"
	"net"
)

type ServerUDP struct {
	listenAddr net.UDPAddr
}

func (x ServerUDP) runAnnouncementListener() {
	conn, err := net.ListenMulticastUDP("udp4", nil, &x.listenAddr)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer conn.Close()

	buffer := make([]byte, 1024)

	fmt.Println("Listening for multicast messages...")
	for {
		n, src, err := conn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Println("Error reading:", err)
			continue
		}
		fmt.Printf("Received message from %s: %s\n", src, string(buffer[:n]))
	}
}
