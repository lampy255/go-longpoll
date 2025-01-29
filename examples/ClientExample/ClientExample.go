package main

import (
	"bufio"
	"fmt"
	"log"
	"os"

	"github.com/lampy255/go-longpoll"
)

func main() {
	// Create a new LongPoll Manager with default settings
	manager := longpoll.NewDefaultManager()

	// Change port to not clash with the ServerExample
	manager.API_Port = 8081

	// Set custom UUID
	manager.UUID = "client1"

	// Set callback functions
	upCallback := func(peerUUID string) {
		log.Println("Peer Up:", peerUUID)
	}
	manager.UpCallback = &upCallback

	downCallback := func(peerUUID string) {
		log.Println("Peer Down:", peerUUID)
	}
	manager.DownCallback = &downCallback

	receiveCallback := func(peerUUID string, message longpoll.Message) {
		fmt.Println("Received Message:", string(message.Data))
	}
	manager.ReceiveCallback = &receiveCallback

	// Start the LongPoll Manager
	err := manager.Start()
	if err != nil {
		log.Fatal(err)
	}

	// Add a server peer
	err = manager.AddServerPeer("server1", "http://localhost:8080/poll", nil, nil)
	if err != nil {
		log.Fatal(err)
	}

	// Take input from CLI and send it to "server1"
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Enter messages to send to the server and press Enter to send")
	for {
		// .Scan() waits for a line to be completed, i.e., the user presses Enter.
		scanned := scanner.Scan()
		if !scanned {
			// If we reach EOF or an error occurred, break
			break
		}

		err := manager.Send("server1", scanner.Text(), nil)
		if err != nil {
			log.Println(err)
		}
	}
}
