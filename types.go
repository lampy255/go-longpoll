package longpoll

import (
	"net/http/cookiejar"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type Manager struct {
	UUID      string
	peers     sync.Map // map[uuid]lpPeer
	cookieJar *cookiejar.Jar

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

type Peer struct {
	UUID             string // Unique identifier for this peer
	ipAddr           string
	Ch               chan Message // Buffered channel for outgoing messages to client peers
	LastConsumed     time.Time    // Last time this client peer consumed a message
	upCallback       *func(string)
	downCallback     *func(string)
	receiveCallback  *func(string, Message)
	Topics           []string          // Topics this peer is subscribed to (see FanOutSubscribers())
	StickyAttrbitues map[string]string // Attributes to be appended to every outgoing message

	// Specific to server peers
	IsServer  bool
	ServerURL string            // URL of server running longpoll API
	Headers   map[string]string // Headers to be applied to outgoing requests
	Online    bool
}
