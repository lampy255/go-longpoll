package longpoll

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"time"
)

// Poll the peer via GET request
func (p *Peer) pollGET(deadline time.Duration, managerUUID string, jar *cookiejar.Jar) error {
	// Create a new request
	req, err := http.NewRequest("GET", p.ServerURL, nil)
	if err != nil {
		p.markOffline()
		return err
	}

	// Set headers
	req.Header.Set("uuid", managerUUID)

	// Set custom headers
	for k, v := range p.Headers {
		req.Header.Set(k, v)
	}

	// Create client
	client := &http.Client{
		Timeout: deadline,
		Jar:     jar,
	}

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		p.markOffline()
		return err
	}

	// Check remote manager UUID
	remoteManagerUUID := resp.Header.Get("uuid")
	if remoteManagerUUID != p.remoteManagerUUID {
		log.Println("Server Peer UUID changed from", p.remoteManagerUUID, "to", remoteManagerUUID)
		p.remoteManagerUUID = remoteManagerUUID
	}

	// Check response code
	switch resp.StatusCode {
	case 200:
		// Read the response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		// Parse the message
		var msg Message
		err = json.Unmarshal(body, &msg)
		if err != nil {
			return err
		}

		// Call the receive callback
		if p.receiveCallback != nil {
			cb := *p.receiveCallback
			go cb(p.UUID, msg)
		}
		p.markOnline()
		return nil
	case 201:
		// Peer created on server
		p.markOnline()
		return nil
	case 204:
		// Poll finished without message
		p.markOnline()
		return nil
	default:
		// Error
		p.markOffline()
		return errors.New("poll failed: " + resp.Status)
	}
}

// Poll the peer via POST request
func (p *Peer) pollPOST(msg Message, managerUUID string, deadline time.Duration, jar *cookiejar.Jar) error {
	// Marshal the message
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// Create the request
	req, err := http.NewRequest("POST", p.ServerURL, bytes.NewReader(msgBytes))
	if err != nil {
		return err
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("uuid", managerUUID)

	// Set custom headers
	for k, v := range p.Headers {
		req.Header.Set(k, v)
	}

	// Create the client
	client := &http.Client{
		Timeout: deadline,
		Jar:     jar,
	}

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	// Check remote manager UUID
	remoteManagerUUID := resp.Header.Get("uuid")
	if remoteManagerUUID != p.remoteManagerUUID {
		log.Println("Server Peer UUID changed from", p.remoteManagerUUID, "to", remoteManagerUUID)
		p.remoteManagerUUID = remoteManagerUUID
	}

	// Check response code
	switch resp.StatusCode {
	case 200:
		return nil
	default:
		return errors.New(resp.Status)
	}
}

func (p *Peer) markOnline() {
	if !p.Online {
		p.Online = true
		if p.upCallback != nil {
			cb := *p.upCallback
			go cb(p.UUID)
		}
	}
}

func (p *Peer) markOffline() {
	if p.Online {
		p.Online = false
		if p.downCallback != nil {
			cb := *p.downCallback
			go cb(p.UUID)
		}
	}
}
