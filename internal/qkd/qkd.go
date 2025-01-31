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
	logger.Info.Println("Starting QKD Peer")

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

	classicalDialer := &net.Dialer{Timeout: 10 * time.Second}
	classicalConn, err := classicalDialer.DialContext(p.ctx, "tcp",
		fmt.Sprintf("%s:%d", p.PeerAddr, p.ClassicalPort))
	if err != nil {
		return fmt.Errorf("classical connection failed: %v", err)
	}
	p.ClassicalConn = classicalConn

	return nil
}

func (p *Peer) initiateQKD() error {
	const numQubits = 256

	bases := make([]int, numQubits)
	bits := make([]int, numQubits)
	for i := range bases {
		bases[i] = cryptoRandBit()
		bits[i] = cryptoRandBit()
	}

	for i := 0; i < numQubits; i++ {
		qubit := bits[i]
		if bases[i] == 1 {
			qubit += 2
		}
		if _, err := fmt.Fprintf(p.QuantumConn, "%d\n", qubit); err != nil {
			return fmt.Errorf("quantum channel send error: %v", err)
		}
	}

	if _, err := fmt.Fprintf(p.ClassicalConn, "%v\n", bases); err != nil {
		return fmt.Errorf("classical channel send error: %v", err)
	}

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

	p.Key = []int{}
	for i := range bases {
		if bases[i] == peerBases[i] {
			if cryptoRandFloat() > 0.95 {
				continue
			}
			p.Key = append(p.Key, bits[i])
		}
	}

	logger.Info.Printf("Generated shared key (%d bits): %v", len(p.Key), p.Key)
	return nil
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
