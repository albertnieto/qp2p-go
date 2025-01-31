package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/albertnieto/qp2p-go/internal/qkd"
	"github.com/albertnieto/qp2p-go/pkg/logger"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage:")
		fmt.Println("  go run qkd.go [local_ip] [peer_ip] [quantum_port] [classical_port]")
		fmt.Println("Example:")
		fmt.Println("  go run qkd.go 127.0.0.1 127.0.0.1 8080 8081")
		return
	}

	localAddr := os.Args[1]
	peerAddr := os.Args[2]

	quantumPort, err := strconv.Atoi(os.Args[3])
	if err != nil {
		fmt.Println("Invalid quantum port")
		return
	}

	classicalPort, err := strconv.Atoi(os.Args[4])
	if err != nil {
		fmt.Println("Invalid classical port")
		return
	}

	logger.Init()

	peer := qkd.NewPeer(localAddr, peerAddr, quantumPort, classicalPort)
	if err := peer.Start(); err != nil {
		fmt.Printf("QKD initialization error: %v\n", err)
	}
}
