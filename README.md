# go-longpoll

A simple go module for HTTP long polling

## Features

- Full duplex communication alternative to WebSockets
- Less susceptible to strict firewalls
- Built in extras eg: Fanout to all peers, topic based subscriptions

## Examples

### Server Example

```go
// Create a new LongPoll Manager with default settings
manager := longpoll.NewDefaultManager()

// Set callback function
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
fmt.Println("Enter messages to send to client1 and press Enter to send")
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

```

### Client Example

```go
// Create a new LongPoll Manager with default settings
manager := longpoll.NewDefaultManager()

// Change port to not clash with the ServerExample
manager.API_Port = 8081

// Set custom UUID
manager.UUID = "client1"

// Set callback function
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
```
