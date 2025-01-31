package main

import (
	"fmt"
	"os"

	"github.com/albertnieto/qkd-p2p/internal/qkd"
	"github.com/albertnieto/qkd-p2p/pkg/logger"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage:")
		fmt.Println("  go run qkd.go [local_ip:port] [peer_ip:port]")
		fmt.Println("Example:")
		fmt.Println("  go run qkd.go :8080 127.0.0.1:8081")
		return
	}

	localAddr := os.Args[1]
	peerAddr := os.Args[2]

	logger.Init()

	peer := qkd.NewPeer(localAddr, peerAddr)
	peer.Start()
}
