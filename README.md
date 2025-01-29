# go-longpoll

A simple bi-directional HTTP long polling package

## Features

- Full duplex communication alternative to WebSockets
- Less susceptible to strict firewalls
- Built in extras eg: Fanout to all peers, topic based subscriptions

## Examples

Client Example

```
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

// Send a message to the server
err := manager.Send("server1", "Hello from client!", nil)
if err != nil {
    log.Println(err)
}
```

Server Example

```
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

// Send a message to peer named "client1"
err := manager.Send("client1", "Hello from server!", nil)
if err != nil {
    log.Println(err)
}

```
