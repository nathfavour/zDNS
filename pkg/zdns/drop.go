package zdns

import (
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"os"

	"golang.org/x/crypto/chacha20poly1305"
)

// FileHeader is the first thing sent over the TCP stream.
type FileHeader struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// SendFile starts a TCP server to serve an encrypted file to a peer.
func SendFile(path string, peer *Peer, listener net.Listener) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	fi, err := file.Stat()
	if err != nil {
		return err
	}

	conn, err := listener.Accept()
	if err != nil {
		return err
	}
	defer conn.Close()

	// Use the shared secret to create an AEAD for the stream
	aead, err := chacha20poly1305.New(peer.SharedSecret)
	if err != nil {
		return err
	}

	// Simple streaming encryption wrapper
	// In a production app, we'd use a more robust streaming protocol like Age
	// For this PoC, we'll send the file in chunks
	nonce := make([]byte, aead.NonceSize())
	// Send nonce first
	if _, err := io.ReadFull(randReader(), nonce); err != nil {
		return err
	}
	conn.Write(nonce)

	buf := make([]byte, 32*1024) // 32KB chunks
	for {
		n, err := file.Read(buf)
		if n > 0 {
			ciphertext := aead.Seal(nil, nonce, buf[:n], nil)
			// Write length prefix
			lenBuf := make([]byte, 4)
			binaryPutUint32(lenBuf, uint32(len(ciphertext)))
			conn.Write(lenBuf)
			conn.Write(ciphertext)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	fmt.Printf("File '%s' (%d bytes) sent to peer.
", fi.Name(), fi.Size())
	return nil
}

// ReceiveFile connects to a sender and receives an encrypted file.
func ReceiveFile(addr string, peer *Peer, fileName string, saveDir string) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	aead, err := chacha20poly1305.New(peer.SharedSecret)
	if err != nil {
		return err
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(conn, nonce); err != nil {
		return err
	}

	outPath := fmt.Sprintf("%s/%s", saveDir, fileName)
	outFile, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	lenBuf := make([]byte, 4)
	for {
		_, err := io.ReadFull(conn, lenBuf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		cLen := binaryUint32(lenBuf)
		ciphertext := make([]byte, cLen)
		if _, err := io.ReadFull(conn, ciphertext); err != nil {
			return err
		}

		plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
		if err != nil {
			return err
		}
		outFile.Write(plaintext)
	}

	fmt.Printf("Received file: %s
", outPath)
	return nil
}

// Helpers to avoid extra imports in this file
func randReader() io.Reader { return rand.Reader }
func binaryPutUint32(b []byte, v uint32) {
	b[0] = byte(v >> 24)
	b[1] = byte(v >> 16)
	b[2] = byte(v >> 8)
	b[3] = byte(v)
}
func binaryUint32(b []byte) uint32 {
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}
