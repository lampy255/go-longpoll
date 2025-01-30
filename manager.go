package longpoll

import (
	"encoding/json"
	"errors"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Creates a new LongPoll Manager with default settings
func NewDefaultManager() *Manager {
	m := &Manager{
		UUID:       uuid.New().String(),
		peers:      sync.Map{},
		API_Port:   8080,
		API_Path:   "/poll",
		PollLength: 10 * time.Second,
		PeerExpiry: 30 * time.Second,
		Deadline:   20 * time.Second,
	}
	return m
}

// Starts the LongPoll Manager API and garbage collection
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
	go r.Run(":" + port)
	return nil
}

// Adds a server peer to the LongPoll Manager
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
	m.peers.Store(uuid, lpp)

	// Start poll routine
	go func() {
		for {
			// Get the peer
			p, _ := m.peers.Load(uuid)
			if p == nil {
				// Quit the routine if the peer has been deleted
				return
			}
			Peer := p.(*Peer)

			// Send Poll (this will block until a message is received)
			err := Peer.poll(m.Deadline, m.UUID)
			if err != nil {
				time.Sleep(m.PollLength)
			}
		}
	}()

	return nil
}

// Deletes a peer from the LongPoll Manager
func (m *Manager) DeletePeer(uuid string) error {
	v, _ := m.peers.Load(uuid)
	if v == nil {
		return errors.New("peer not found")
	}
	peer := v.(*Peer)

	// Close channel if it exists
	if peer.Ch != nil {
		close(peer.Ch)
	}

	// Delete the peer
	m.peers.Delete(uuid)
	return nil
}

// Check if a peer exists
func (m *Manager) PeerExists(uuid string) bool {
	v, _ := m.peers.Load(uuid)
	return v != nil
}

// Adds a topic to a peer
func (m *Manager) AddTopic(uuid string, topic string) error {
	lpp, _ := m.peers.Load(uuid)
	if lpp == nil {
		return errors.New("peer not found")
	}

	peer := lpp.(*Peer)

	peer.Topics = append(peer.Topics, topic)
	return nil
}

// Removes a topic from a peer
func (m *Manager) RemoveTopic(uuid string, topic string) error {
	lpp, _ := m.peers.Load(uuid)
	if lpp == nil {
		return errors.New("peer not found")
	}

	peer := lpp.(*Peer)

	topics := peer.Topics
	for i, t := range topics {
		if t == topic {
			peer.Topics = append(topics[:i], topics[i+1:]...)
			break
		}
	}
	return nil
}

// Gets the topics of a peer
func (m *Manager) GetTopics(uuid string) ([]string, error) {
	lpp, _ := m.peers.Load(uuid)
	if lpp == nil {
		return nil, errors.New("peer not found")
	}

	peer := lpp.(*Peer)
	return peer.Topics, nil
}

// Sets the topics of a peer
func (m *Manager) SetTopics(uuid string, topics []string) error {
	lpp, _ := m.peers.Load(uuid)
	if lpp == nil {
		return errors.New("peer not found")
	}

	peer := lpp.(*Peer)
	peer.Topics = topics
	return nil
}

// Gets the IP address of a peer
func (m *Manager) GetPeerIP(uuid string) (string, error) {
	lpp, _ := m.peers.Load(uuid)
	if lpp == nil {
		return "", errors.New("peer not found")
	}

	peer := lpp.(*Peer)
	return peer.ipAddr, nil
}

// Sets the sticky attributes of a peer
func (m *Manager) SetPeerStickyAttributes(peerUUID string, attributes map[string]string) error {
	lpp, _ := m.peers.Load(peerUUID)
	if lpp == nil {
		return errors.New("peer not found")
	}

	peer := lpp.(*Peer)
	peer.StickyAttrbitues = attributes
	return nil
}

// Sends a message to a peer
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
	lpp, _ := m.peers.Load(peerUUID)
	if lpp == nil {
		return errors.New("failed to send message to " + peerUUID + ": peer not found")
	}

	// Cast the peer
	peer := lpp.(*Peer)

	// Apply sticky attributes
	for k, v := range peer.StickyAttrbitues {
		message.Attributes[k] = v
	}

	// Check if the peer is a server
	if peer.IsServer {
		// Send via POST
		err := m.sendPOST(peer.ServerURL, message, peer.Headers)
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
			return errors.New("failed to send message to " + peerUUID + ": deadline exceeded")
		}
	}
}

// Forwards an existing message to a peer
func (m *Manager) Forward(peerUUID string, message Message) error {
	// Retrieve the peer
	lpp, _ := m.peers.Load(peerUUID)
	if lpp == nil {
		return errors.New("failed to forward message to " + peerUUID + ": peer not found")
	}

	// Cast the peer
	peer := lpp.(*Peer)

	// Check if the peer is a server
	if peer.IsServer {
		// Send via POST
		err := m.sendPOST(peer.ServerURL, message, peer.Headers)
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

// Sends a message to all peers
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

	// Create a new message
	message := Message{
		Data:        dataBytes,
		Attributes:  attributes,
		MessageID:   uuid.New().String(),
		PublishTime: time.Now(),
	}

	// Send the message to all peers
	m.peers.Range(func(key, value interface{}) bool {
		peer := value.(*Peer)

		// Check if the peer is a server
		if peer.IsServer {
			// Send via POST
			err := m.sendPOST(peer.ServerURL, message, peer.Headers)
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
					log.Println("failed to FanOut message to peer:", key)
				}
			}()
		}

		return true
	})

	return nil
}

// Sends a message to all peers subscribed to a given topic
func (m *Manager) FanOutSubscribers(data interface{}, atttributes map[string]string, topic string) error {
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

	// Create a new message
	message := Message{
		Data:        dataBytes,
		Attributes:  atttributes,
		MessageID:   uuid.New().String(),
		PublishTime: time.Now(),
	}

	// Send the message to all subscribers
	m.peers.Range(func(key, value interface{}) bool {
		peer := value.(*Peer)

		// Check if the peer is subscribed to the topic
		for _, t := range peer.Topics {
			if t == topic {
				// Check if the peer is a server
				if peer.IsServer {
					// Send via POST
					err := m.sendPOST(peer.ServerURL, message, peer.Headers)
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
							log.Println("failed to FanOut message to peer:", key)
						}
					}()
				}
				break
			}
		}

		return true
	})

	return nil
}
