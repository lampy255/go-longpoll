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

	// Take input from CLI and send it to "client1"
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Enter messages to send to all clients and press Enter to send")
	for {
		// .Scan() waits for a line to be completed, i.e., the user presses Enter.
		scanned := scanner.Scan()
		if !scanned {
			// If we reach EOF or an error occurred, break
			break
		}

		err := manager.Send("client1", scanner.Text(), nil)
		if err != nil {
			log.Println(err)
		}
	}
}
