package qkd

import (
	"bufio"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"strings"

	"github.com/albertnieto/qp2p-go/pkg/logger"
)

const (
	QuantumPort   = ":8080"
	ClassicalPort = ":8081"
)

type Peer struct {
	Address       string
	QuantumConn   net.Conn
	ClassicalConn net.Conn
	Key           []int
}

func NewPeer(localAddr, peerAddr string) *Peer {
	return &Peer{
		Address: localAddr,
	}
}

func (p *Peer) Start() {
	logger.Info.Println("Starting QKD Peer")

	go p.startQuantumServer()
	go p.startClassicalServer()

	p.connectToPeer(p.Address)

	p.initiateQKD()
}

func (p *Peer) startQuantumServer() {
	ln, err := net.Listen("tcp", QuantumPort)
	if err != nil {
		logger.Error.Fatalf("Error starting quantum server: %v", err)
	}
	defer ln.Close()

	conn, err := ln.Accept()
	if err != nil {
		logger.Error.Fatalf("Error accepting quantum connection: %v", err)
	}
	p.QuantumConn = conn
	logger.Info.Println("Quantum channel established")
}

func (p *Peer) startClassicalServer() {
	ln, err := net.Listen("tcp", ClassicalPort)
	if err != nil {
		logger.Error.Fatalf("Error starting classical server: %v", err)
	}
	defer ln.Close()

	conn, err := ln.Accept()
	if err != nil {
		logger.Error.Fatalf("Error accepting classical connection: %v", err)
	}
	p.ClassicalConn = conn
	logger.Info.Println("Classical channel established")
}

func (p *Peer) connectToPeer(peerAddr string) {
	// Connect to quantum channel
	quantumConn, err := net.Dial("tcp", peerAddr+QuantumPort)
	if err != nil {
		logger.Error.Printf("Quantum connection failed: %v", err)
		return
	}
	p.QuantumConn = quantumConn

	// Connect to classical channel
	classicalConn, err := net.Dial("tcp", peerAddr+ClassicalPort)
	if err != nil {
		logger.Error.Printf("Classical connection failed: %v", err)
		return
	}
	p.ClassicalConn = classicalConn
}

func (p *Peer) initiateQKD() {
	const numQubits = 256

	// Generate random bases and bits
	bases := make([]int, numQubits)
	bits := make([]int, numQubits)
	for i := range bases {
		bases[i] = rand.Intn(2)
		bits[i] = rand.Intn(2)
	}

	// Send qubits over quantum channel
	for i := 0; i < numQubits; i++ {
		qubit := bits[i]
		if bases[i] == 1 { // Use diagonal basis
			qubit += 2
		}
		fmt.Fprintf(p.QuantumConn, "%d\n", qubit)
	}

	// Send bases over classical channel
	fmt.Fprintf(p.ClassicalConn, "%v\n", bases)

	// Receive peer's bases
	peerBasesStr, _ := bufio.NewReader(p.ClassicalConn).ReadString('\n')
	var peerBases []int
	for _, s := range strings.Fields(strings.TrimSpace(peerBasesStr)) {
		val, _ := strconv.Atoi(s)
		peerBases = append(peerBases, val)
	}

	// Generate shared key
	for i := range bases {
		if bases[i] == peerBases[i] {
			p.Key = append(p.Key, bits[i])
		}
	}

	logger.Info.Printf("Generated shared key (%d bits): %v", len(p.Key), p.Key)
}
