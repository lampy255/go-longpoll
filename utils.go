package longpoll

import (
	"encoding/json"
	"io"
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

	// Set manager UUID in response headers
	c.Header("uuid", m.UUID)

	// Does the peer exist?
	m.peersMU.RLock()
	peer, _ := m.peers[uuid]
	m.peersMU.RUnlock()
	if peer == nil {
		// Create a new peer
		ch := make(chan Message, m.OutboundBufferSize)
		newPeer := &Peer{
			UUID:            uuid,
			Ch:              ch,
			Online:          true,
			LastConsumed:    time.Now(),
			upCallback:      m.UpCallback,
			downCallback:    m.DownCallback,
			receiveCallback: m.ReceiveCallback,
		}
		m.peersMU.Lock()
		m.peers[uuid] = newPeer
		m.peersMU.Unlock()
		peer = newPeer

		// Call the manager up callback
		if m.UpCallback != nil {
			cb := *m.UpCallback
			go cb(uuid)
		}

		// Reply 201 to indicate that the peer has been created
		c.Status(201)
		return
	}

	// Update the peer ipAddress
	peer.ipAddr = c.ClientIP()

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

	// Set manager UUID in response headers
	c.Header("uuid", m.UUID)

	// Does the peer exist?
	m.peersMU.RLock()
	peer, _ := m.peers[uuid]
	m.peersMU.RUnlock()
	if peer == nil {
		// Create a new peer
		ch := make(chan Message, m.OutboundBufferSize)
		newPeer := &Peer{
			UUID:            uuid,
			ipAddr:          c.ClientIP(),
			Online:          true,
			Ch:              ch,
			LastConsumed:    time.Now(),
			upCallback:      m.UpCallback,
			downCallback:    m.DownCallback,
			receiveCallback: m.ReceiveCallback,
		}
		m.peersMU.Lock()
		m.peers[uuid] = newPeer
		m.peersMU.Unlock()
		peer = newPeer

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

// Deletes peers that have expired
func (m *Manager) garbageCollectPeers() {
	m.peersMU.Lock()
	defer m.peersMU.Unlock()
	for key, peer := range m.peers {
		// Skip servers
		if peer.IsServer {
			continue
		}

		// Check if the peer has expired
		if time.Since(peer.LastConsumed) > m.PeerExpiry {
			if m.DownCallback != nil {
				cb := *m.DownCallback
				go cb(peer.UUID)
			}
			close(peer.Ch)
			delete(m.peers, key)
		}
	}
}

func stringPlaceHolder(s string) string {
	if s == "" {
		return "none"
	}
	return s
}
