package longpoll

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
)

func (p *lpPeer) poll(deadline time.Duration, managerUUID string) error {
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
		if p.ReceiveCallback != nil {
			cb := *p.ReceiveCallback
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

func (p *lpPeer) markOnline() {
	if !p.Online {
		p.Online = true
		if p.UpCallback != nil {
			cb := *p.UpCallback
			go cb(p.UUID)
		}
	}
}

func (p *lpPeer) markOffline() {
	if p.Online {
		p.Online = false
		if p.DownCallback != nil {
			cb := *p.DownCallback
			go cb(p.UUID)
		}
	}
}
