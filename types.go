package longpoll

import (
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type Manager struct {
	UUID  string
	peers sync.Map // map[uuid]lpPeer

	API_Port       int              // Port to listen on
	API_Path       string           // Path to listen on eg: /poll
	API_Middleware *gin.HandlerFunc // Middleware to run before each request
	PollLength     time.Duration    // Time before a poll should be refreshed
	PeerExpiry     time.Duration    // Time before a peer is considered expired/offline
	Deadline       time.Duration    // Time before a poll times out

	UpCallback      *func(peerUUID string)              // Function to call when a peer comes online
	DownCallback    *func(peerUUID string)              // Function to call when a peer goes offline
	ReceiveCallback *func(peerUUID string, msg Message) // Function to call when receiving a message
}

type Message struct {
	Data        []byte            `json:"data"`
	Attributes  map[string]string `json:"attributes"`
	MessageID   string            `json:"message_id"`
	PublishTime time.Time         `json:"publish_time"`
}

type lpPeer struct {
	UUID             string
	Ch               chan Message
	LastConsumed     time.Time
	UpCallback       *func(string)
	DownCallback     *func(string)
	ReceiveCallback  *func(string, Message)
	Topics           []string
	StickyAttrbitues map[string]string

	// Specific to server peers
	IsServer  bool
	ServerURL string
	Headers   map[string]string
	Online    bool
}
