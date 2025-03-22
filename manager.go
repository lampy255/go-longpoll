package longpoll

import (
	"encoding/json"
	"errors"
	"log"
	"net/http/cookiejar"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// NewDefaultManager Creates a new LongPoll Manager with default settings
func NewDefaultManager() *Manager {
	jar, err := cookiejar.New(nil)
	if err != nil {
		log.Fatalf("Failed to create cookie jar: %v", err)
	}

	m := &Manager{
		UUID:       uuid.New().String(),
		cookieJar:  jar,
		peers:      make(map[string]*Peer, 255),
		API_Port:   8080,
		API_Path:   "/poll",
		PollLength: 10 * time.Second,
		PeerExpiry: 30 * time.Second,
		Deadline:   20 * time.Second,
	}
	return m
}

// Start Starts the LongPoll Manager API and garbage collection
func (m *Manager) Start() error {
	// Dummy checks
	if m.API_Path == "" {
		return errors.New("API_Path is required")
	}
	if m.PollLength < 1*time.Second {
		return errors.New("PollLength must be at least 1 second")
	}
	if m.PeerExpiry < 1*time.Second {
		return errors.New("PeerExpiry must be at least 1 second")
	}
	if m.Deadline < 1*time.Second {
		return errors.New("deadline must be at least 1 second")
	}

	// Convert port to string
	port := strconv.Itoa(m.API_Port)

	// Start Garbage Collection
	go func() {
		for {
			m.garbageCollectPeers()
			time.Sleep(10 * time.Second)
		}
	}()

	// Create API server
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	// Apply Middleware if applicable
	if m.API_Middleware != nil {
		r.Use(*m.API_Middleware)
	}

	// Add routes
	r.GET(m.API_Path, m.handleGET)
	r.POST(m.API_Path, m.handlePOST)

	// Start the server
	go func() {
		err := r.Run(":" + port)
		if err != nil {
			log.Fatalf("Failed to start API server: %v", err)
		}
	}()
	return nil
}

// AddServerPeer Adds a server peer to the LongPoll Manager
func (m *Manager) AddServerPeer(uuid string, url string, headers map[string]string, stickyAttributes map[string]string) error {
	// Check uuid is not empty
	if uuid == "" {
		return errors.New("uuid is required")
	}

	// Check server URL is not empty
	if url == "" {
		return errors.New("server URL is required")
	}

	// Does the peer already exist?
	if m.PeerExists(uuid) {
		return errors.New("peer already exists")
	}

	// Create a new Peer
	lpp := &Peer{
		UUID:             uuid,
		IsServer:         true,
		ServerURL:        url,
		Headers:          headers,
		StickyAttrbitues: stickyAttributes,
		upCallback:       m.UpCallback,
		downCallback:     m.DownCallback,
		receiveCallback:  m.ReceiveCallback,
	}

	// Store the peer
	m.peersMU.Lock()
	m.peers[uuid] = lpp
	m.peersMU.Unlock()

	// Start poll routine
	go func() {
		for {
			// Get the peer
			m.peersMU.RLock()
			Peer, _ := m.peers[uuid]
			m.peersMU.RUnlock()
			if Peer == nil {
				// Quit the routine if the peer has been deleted
				return
			}

			// Send Poll (this will block until a message is received)
			err := Peer.pollGET(m.Deadline, m.UUID, m.cookieJar)
			if err != nil {
				time.Sleep(m.PollLength)
			}
		}
	}()

	return nil
}

// DeletePeer Deletes a peer from the LongPoll Manager
func (m *Manager) DeletePeer(uuid string) error {
	m.peersMU.Lock()
	defer m.peersMU.Unlock()
	peer, _ := m.peers[uuid]
	if peer == nil {
		return errors.New("peer not found")
	}

	// Close channel if it exists
	if peer.Ch != nil {
		close(peer.Ch)
	}

	// Delete the peer
	delete(m.peers, uuid)
	return nil
}

// PeerExists Checks if a peer exists. This function locks peersMU!
func (m *Manager) PeerExists(uuid string) bool {
	m.peersMU.RLock()
	defer m.peersMU.RUnlock()
	peer, _ := m.peers[uuid]
	return peer != nil
}

// AddTopic Adds a topic to a peer
func (m *Manager) AddTopic(uuid string, topic string) error {
	m.peersMU.Lock()
	defer m.peersMU.Unlock()
	peer, _ := m.peers[uuid]
	if peer == nil {
		return errors.New("peer not found")
	}

	peer.Topics = append(peer.Topics, topic)
	return nil
}

// RemoveTopic Removes a topic from a peer
func (m *Manager) RemoveTopic(uuid string, topic string) error {
	m.peersMU.Lock()
	defer m.peersMU.Unlock()
	peer, _ := m.peers[uuid]
	if peer == nil {
		return errors.New("peer not found")
	}

	topics := peer.Topics
	for i, t := range topics {
		if t == topic {
			peer.Topics = append(topics[:i], topics[i+1:]...)
			break
		}
	}
	return nil
}

// GetTopics Gets the topics of a peer
func (m *Manager) GetTopics(uuid string) ([]string, error) {
	m.peersMU.RLock()
	defer m.peersMU.RUnlock()
	peer, _ := m.peers[uuid]
	if peer == nil {
		return nil, errors.New("peer not found")
	}

	return peer.Topics, nil
}

// SetTopics Sets the topics of a peer
func (m *Manager) SetTopics(uuid string, topics []string) error {
	m.peersMU.Lock()
	defer m.peersMU.Unlock()

	peer, _ := m.peers[uuid]
	if peer == nil {
		return errors.New("peer not found")
	}

	peer.Topics = topics
	return nil
}

// GetPeerIP Gets the IP address of a peer
func (m *Manager) GetPeerIP(uuid string) (string, error) {
	m.peersMU.RLock()
	defer m.peersMU.RUnlock()
	peer, _ := m.peers[uuid]
	if peer == nil {
		return "", errors.New("peer not found")
	}

	return peer.ipAddr, nil
}

// SetPeerStickyAttributes Sets the sticky attributes of a peer
func (m *Manager) SetPeerStickyAttributes(peerUUID string, attributes map[string]string) error {
	m.peersMU.Lock()
	defer m.peersMU.Unlock()
	peer, _ := m.peers[peerUUID]
	if peer == nil {
		return errors.New("peer not found")
	}

	peer.StickyAttrbitues = attributes
	return nil
}

// Send Sends a message to a peer. Locks Mutex!
func (m *Manager) Send(peerUUID string, data interface{}, attributes map[string]string) error {
	// Marshal the data
	var dataBytes []byte
	if data != nil {
		bytes, err := json.Marshal(data)
		if err != nil {
			return errors.New("failed to send message to " + peerUUID + ": error marshalling data")
		}

		dataBytes = bytes
	} else {
		dataBytes = []byte{}
	}

	// Create a new message
	message := Message{
		Data:        dataBytes,
		Attributes:  attributes,
		MessageID:   uuid.New().String(),
		PublishTime: time.Now(),
	}

	// Retrieve the peer
	m.peersMU.RLock()
	peer, _ := m.peers[peerUUID]
	m.peersMU.RUnlock()
	if peer == nil {
		return errors.New("failed to send message to " + peerUUID + ": peer not found")
	}

	// Apply sticky attributes
	for k, v := range peer.StickyAttrbitues {
		message.Attributes[k] = v
	}

	// Check if the peer is a server
	if peer.IsServer {
		// Send via POST
		err := peer.pollPOST(message, m.UUID, m.Deadline, m.cookieJar)
		if err != nil {
			return errors.New("failed to send message to " + peerUUID + ": " + err.Error())
		} else {
			return nil
		}
	} else {
		// Send via channel
		select {
		case peer.Ch <- message:
			return nil
		case <-time.After(m.Deadline):
			return errors.New("failed to send message to peer: " + peerUUID + ": deadline exceeded")
		}
	}
}

// Forward Forwards an existing message to a peer. Locks Mutex!
func (m *Manager) Forward(peerUUID string, message Message) error {
	// Retrieve the peer
	m.peersMU.RLock()
	peer, _ := m.peers[peerUUID]
	m.peersMU.RUnlock()
	if peer == nil {
		return errors.New("failed to forward message to " + peerUUID + ": peer not found")
	}

	// Apply sticky attributes
	for k, v := range peer.StickyAttrbitues {
		message.Attributes[k] = v
	}

	// Check if the peer is a server
	if peer.IsServer {
		// Send via POST
		err := peer.pollPOST(message, m.UUID, m.Deadline, m.cookieJar)
		if err != nil {
			return errors.New("failed to forward message to " + peerUUID + ": " + err.Error())
		} else {
			return nil
		}
	} else {
		// Send via channel
		select {
		case peer.Ch <- message:
			return nil
		case <-time.After(m.Deadline):
			return errors.New("failed to forward message to " + peerUUID + ": deadline exceeded")
		}
	}
}

// FanOut Sends a message to all peers
func (m *Manager) FanOut(data interface{}, attributes map[string]string) error {
	// Marshal the data
	var dataBytes []byte
	if data != nil {
		bytes, err := json.Marshal(data)
		if err != nil {
			return err
		}

		dataBytes = bytes
	} else {
		dataBytes = []byte{}
	}

	// Send the message to all peers
	m.peersMU.Lock()
	defer m.peersMU.Unlock()
	for key, peer := range m.peers {
		// Skip peers that are offline
		if !peer.Online {
			continue
		}

		// Create a new message
		message := Message{
			Data:        dataBytes,
			Attributes:  attributes,
			MessageID:   uuid.New().String(),
			PublishTime: time.Now(),
		}

		// Apply sticky attributes
		for k, v := range peer.StickyAttrbitues {
			message.Attributes[k] = v
		}

		// Check if the peer is a server
		if peer.IsServer {
			// Send via POST
			err := peer.pollPOST(message, m.UUID, m.Deadline, m.cookieJar)
			if err != nil {
				log.Println("failed to FanOut message to " + peer.UUID + ": " + err.Error())
			}
		} else {
			// Send the message to the peers channel
			go func() {
				select {
				case peer.Ch <- message:
					return
				case <-time.After(m.Deadline):
					log.Println("failed to FanOut message to peer:", key, "deadline exceeded")
					return
				default:
					log.Println("failed to FanOut message to peer:", key, peer.ipAddr)
				}
			}()
		}
	}

	return nil
}

// FanOutSubscribers Sends a message to all peers subscribed to a given topic
func (m *Manager) FanOutSubscribers(data interface{}, attributes map[string]string, topic string) error {
	// Marshal the data
	var dataBytes []byte
	if data != nil {
		bytes, err := json.Marshal(data)
		if err != nil {
			return err
		}

		dataBytes = bytes
	} else {
		dataBytes = []byte{}
	}

	// Send the message to all subscribers
	m.peersMU.Lock()
	defer m.peersMU.Unlock()
	for key, peer := range m.peers {
		// Skip peers that are offline
		if !peer.Online {
			continue
		}

		// Create a new message
		message := Message{
			Data:        dataBytes,
			Attributes:  attributes,
			MessageID:   uuid.New().String(),
			PublishTime: time.Now(),
		}

		// Apply sticky attributes
		for k, v := range peer.StickyAttrbitues {
			message.Attributes[k] = v
		}

		// Check if the peer is subscribed to the topic
		for _, t := range peer.Topics {
			if t == topic {
				// Check if the peer is a server
				if peer.IsServer {
					// Send via POST
					err := peer.pollPOST(message, m.UUID, m.Deadline, m.cookieJar)
					if err != nil {
						log.Println("failed to FanOut message to " + peer.UUID + ": " + err.Error())
					}
				} else {
					go func() {
						// Send via channel
						select {
						case peer.Ch <- message:
							return
						case <-time.After(m.Deadline):
							log.Println("failed to FanOut message to peer:", key, ": deadline exceeded")
							return
						}
					}()
				}
				break
			}
		}
	}

	return nil
}
