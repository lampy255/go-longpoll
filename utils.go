package longpoll

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func (m *Manager) handleGET(c *gin.Context) {
	// Get the peer UUID
	uuid := c.Request.Header.Get("uuid")
	if uuid == "" {
		c.JSON(400, gin.H{
			"error": "uuid is required",
		})
		return
	}

	// Does the peer exist?
	lpp, _ := m.peers.Load(uuid)
	if lpp == nil {
		// Create a new peer
		ch := make(chan Message, 50)
		newPeer := &Peer{
			UUID:            uuid,
			Ch:              ch,
			LastConsumed:    time.Now(),
			upCallback:      m.UpCallback,
			downCallback:    m.DownCallback,
			receiveCallback: m.ReceiveCallback,
		}
		m.peers.Store(uuid, newPeer)
		lpp = newPeer

		// Call the manager up callback
		if m.UpCallback != nil {
			cb := *m.UpCallback
			go cb(uuid)
		}

		// Reply 201 to indicate that the peer has been created
		c.Status(201)
		return
	}

	// Cast the peer
	peer := lpp.(*Peer)

	// Send available message or wait
	select {
	case msg := <-peer.Ch:
		peer.LastConsumed = time.Now()
		c.JSON(200, msg)
		return
	case <-time.After(m.PollLength):
		peer.LastConsumed = time.Now()
		c.Status(204)
		return
	case <-c.Request.Context().Done():
		return
	}
}

func (m *Manager) handlePOST(c *gin.Context) {
	// Get the peer UUID
	uuid := c.Request.Header.Get("uuid")
	if uuid == "" {
		c.JSON(400, gin.H{
			"error": "uuid is required",
		})
		return
	}

	// Does the peer exist?
	lpp, _ := m.peers.Load(uuid)
	if lpp == nil {
		// Create a new peer
		ch := make(chan Message, 50)
		newPeer := &Peer{
			UUID:            uuid,
			Ch:              ch,
			LastConsumed:    time.Now(),
			upCallback:      m.UpCallback,
			downCallback:    m.DownCallback,
			receiveCallback: m.ReceiveCallback,
		}
		m.peers.Store(uuid, newPeer)
		lpp = newPeer

		// Call the manager up callback
		if m.UpCallback != nil {
			cb := *m.UpCallback
			go cb(uuid)
		}
	}

	// Read the request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "failed to read request body",
		})
		return
	}

	// Parse the message
	var msg Message
	err = json.Unmarshal(body, &msg)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "failed to parse message",
		})
		return
	}

	// Call the manager receive callback
	if m.ReceiveCallback != nil {
		cb := *m.ReceiveCallback
		go cb(uuid, msg)
	}
	c.Status(200)
}

// Send a POST request to a server peer
func (m *Manager) sendPOST(url string, msg Message, headers map[string]string) error {
	// Marshal the message
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// Create the request
	req, err := http.NewRequest("POST", url, bytes.NewReader(msgBytes))
	if err != nil {
		return err
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("uuid", m.UUID)

	// Set custom headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// Create the client
	client := &http.Client{
		Timeout: m.Deadline,
	}

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	// Check response code
	switch resp.StatusCode {
	case 200:
		return nil
	default:
		return errors.New(resp.Status)
	}
}

// Deletes peers that have expired
func (m *Manager) garbageCollectPeers() {
	m.peers.Range(func(key, value interface{}) bool {
		// Cast the value to a lpPeer
		peer := value.(*Peer)

		// Skip servers
		if peer.IsServer {
			return true
		}

		// Check if the peer has expired
		if time.Since(peer.LastConsumed) > m.PeerExpiry {
			if m.DownCallback != nil {
				cb := *m.DownCallback
				go cb(peer.UUID)
			}
			close(peer.Ch)
			m.peers.Delete(key)
		}
		return true
	})
}
