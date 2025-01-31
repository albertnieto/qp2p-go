package qkd

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/albertnieto/qp2p-go/pkg/logger"
)

type Peer struct {
	LocalAddr     string
	PeerAddr      string
	QuantumConn   net.Conn
	ClassicalConn net.Conn
	Key           []int
	QuantumPort   int
	ClassicalPort int
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
}

func NewPeer(localAddr, peerAddr string, quantumPort, classicalPort int) *Peer {
	ctx, cancel := context.WithCancel(context.Background())
	return &Peer{
		LocalAddr:     localAddr,
		PeerAddr:      peerAddr,
		QuantumPort:   quantumPort,
		ClassicalPort: classicalPort,
		ctx:           ctx,
		cancel:        cancel,
	}
}

func (p *Peer) Start() error {
	logger.Info.Printf("Starting QKD Peer | Local: %s:%d | Peer: %s:%d",
		p.LocalAddr, p.QuantumPort, p.PeerAddr, p.ClassicalPort)

	errChan := make(chan error, 2)
	go func() {
		if err := p.startQuantumServer(); err != nil {
			errChan <- fmt.Errorf("quantum server error: %v", err)
		}
	}()

	go func() {
		if err := p.startClassicalServer(); err != nil {
			errChan <- fmt.Errorf("classical server error: %v", err)
		}
	}()

	if err := p.connectToPeer(); err != nil {
		return err
	}

	select {
	case err := <-errChan:
		return err
	case <-time.After(10 * time.Second):
		return p.initiateQKD()
	}
}

func (p *Peer) startQuantumServer() error {
	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", p.LocalAddr, p.QuantumPort))
	if err != nil {
		return fmt.Errorf("quantum server listen error: %v", err)
	}
	defer ln.Close()

	logger.Info.Printf("Quantum server listening on %s:%d", p.LocalAddr, p.QuantumPort)
	conn, err := ln.Accept()
	if err != nil {
		return fmt.Errorf("quantum connection accept error: %v", err)
	}
	p.QuantumConn = conn
	logger.Info.Println("Quantum channel established")
	return nil
}

func (p *Peer) startClassicalServer() error {
	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", p.LocalAddr, p.ClassicalPort))
	if err != nil {
		return fmt.Errorf("classical server listen error: %v", err)
	}
	defer ln.Close()

	logger.Info.Printf("Classical server listening on %s:%d", p.LocalAddr, p.ClassicalPort)
	conn, err := ln.Accept()
	if err != nil {
		return fmt.Errorf("classical connection accept error: %v", err)
	}
	p.ClassicalConn = conn
	logger.Info.Println("Classical channel established")
	return nil
}

func (p *Peer) connectToPeer() error {
	quantumDialer := &net.Dialer{Timeout: 10 * time.Second}
	quantumConn, err := quantumDialer.DialContext(p.ctx, "tcp",
		fmt.Sprintf("%s:%d", p.PeerAddr, p.QuantumPort))
	if err != nil {
		return fmt.Errorf("quantum connection failed: %v", err)
	}
	p.QuantumConn = quantumConn
	logger.Info.Printf("Connected to peer quantum channel at %s:%d", p.PeerAddr, p.QuantumPort)

	classicalDialer := &net.Dialer{Timeout: 10 * time.Second}
	classicalConn, err := classicalDialer.DialContext(p.ctx, "tcp",
		fmt.Sprintf("%s:%d", p.PeerAddr, p.ClassicalPort))
	if err != nil {
		return fmt.Errorf("classical connection failed: %v", err)
	}
	p.ClassicalConn = classicalConn
	logger.Info.Printf("Connected to peer classical channel at %s:%d", p.PeerAddr, p.ClassicalPort)

	return nil
}

func (p *Peer) initiateQKD() error {
	const numQubits = 256
	logger.Info.Printf("Initiating Quantum Key Distribution with %d qubits", numQubits)

	// Generate random bases and bits
	bases := make([]int, numQubits)
	bits := make([]int, numQubits)
	for i := range bases {
		bases[i] = cryptoRandBit()
		bits[i] = cryptoRandBit()
	}
	logger.Info.Println("Generated random bases and bits for quantum transmission")

	// Send qubits over quantum channel
	logger.Info.Println("Transmitting qubits over quantum channel")
	for i := 0; i < numQubits; i++ {
		qubit := bits[i]
		if bases[i] == 1 {
			qubit += 2
		}
		if _, err := fmt.Fprintf(p.QuantumConn, "%d\n", qubit); err != nil {
			return fmt.Errorf("quantum channel send error: %v", err)
		}
		logger.Info.Printf("Sent qubit %d (basis: %d, value: %d)", i, bases[i], qubit)
	}

	// Send bases over classical channel
	logger.Info.Println("Transmitting measurement bases over classical channel")
	if _, err := fmt.Fprintf(p.ClassicalConn, "%v\n", bases); err != nil {
		return fmt.Errorf("classical channel send error: %v", err)
	}

	// Receive peer's bases
	logger.Info.Println("Waiting to receive peer's measurement bases")
	peerBasesStr, err := bufio.NewReader(p.ClassicalConn).ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading peer bases error: %v", err)
	}

	var peerBases []int
	for _, s := range strings.Fields(strings.TrimSpace(peerBasesStr)) {
		val, err := strconv.Atoi(s)
		if err != nil {
			logger.Error.Printf("Error parsing peer basis: %v", err)
			continue
		}
		peerBases = append(peerBases, val)
	}
	logger.Info.Println("Received peer's measurement bases")

	// Generate shared key
	p.Key = []int{}
	logger.Info.Println("Generating shared key")
	for i := range bases {
		if bases[i] == peerBases[i] {
			if cryptoRandFloat() > 0.95 {
				logger.Info.Printf("Discarded qubit %d due to uncertainty", i)
				continue
			}
			p.Key = append(p.Key, bits[i])
			logger.Info.Printf("Added bit %d to shared key", bits[i])
		}
	}

	logger.Info.Printf("Generated shared key (%d bits): %v", len(p.Key), p.Key)
	return nil
}

// Method to send messages via classical channel
func (p *Peer) SendMessage(message string) error {
	if p.ClassicalConn == nil {
		return fmt.Errorf("classical channel not established")
	}

	logger.Info.Printf("Sending message: %s", message)
	_, err := fmt.Fprintf(p.ClassicalConn, "%s\n", message)
	return err
}

// Method to receive messages via classical channel
func (p *Peer) ReceiveMessage() (string, error) {
	if p.ClassicalConn == nil {
		return "", fmt.Errorf("classical channel not established")
	}

	message, err := bufio.NewReader(p.ClassicalConn).ReadString('\n')
	if err != nil {
		return "", err
	}

	logger.Info.Printf("Received message: %s", strings.TrimSpace(message))
	return strings.TrimSpace(message), nil
}

func cryptoRandBit() int {
	b := make([]byte, 1)
	rand.Read(b)
	return int(b[0] % 2)
}

func cryptoRandFloat() float64 {
	b := make([]byte, 8)
	rand.Read(b)
	return math.Abs(math.Float64frombits(binary.BigEndian.Uint64(b))) / math.MaxFloat64
}

func (p *Peer) Close() {
	p.cancel()
	if p.QuantumConn != nil {
		p.QuantumConn.Close()
	}
	if p.ClassicalConn != nil {
		p.ClassicalConn.Close()
	}
}
