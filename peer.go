package longpoll

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
)

func (p *Peer) poll(deadline time.Duration, managerUUID string) error {
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
	}

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		p.markOffline()
		return err
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
