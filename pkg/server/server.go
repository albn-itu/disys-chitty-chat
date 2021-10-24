package server

import (
	"log"
	"sync"

	"github.com/google/uuid"

	pb "github.com/goose-alt/chitty-chat/api/v1/pb/chat"
	"github.com/goose-alt/chitty-chat/internal"
	"github.com/goose-alt/chitty-chat/internal/logging"
)

type chatServer struct {
	pb.UnimplementedChatServer

	// List of clients, mapped by their generated id
	clients map[string]*internal.Client
	logger  logging.Log
	lock    sync.Mutex
}

func NewChatServer() chatServer {
	return chatServer{
		clients: make(map[string]*internal.Client),
		logger:  logging.New(),
	}
}

func (s *chatServer) addClient(stream pb.Chat_ChatServer) *internal.Client {
	s.lock.Lock()
	defer s.lock.Unlock()

	// Generate a new uuid for the client
	id := uuid.New().String()

	// TODO: Replace name with username
	client := internal.Client{
		Uuid: id,
		Name: "",
		Chat: stream,
	}

	s.clients[id] = &client

	return &client
}

func (s *chatServer) removeClient(client *internal.Client) {
	s.lock.Lock()
	defer s.lock.Unlock()

	delete(s.clients, client.Uuid)
}

/*
Is a stream to send chat messages. This is bidirectional.

The implementation is inspired by: https://github.com/castaneai/grpc-broadcast-example/blob/master/server/server.go
*/
func (s *chatServer) Chat(stream pb.Chat_ChatServer) error {
	client := s.addClient(stream) // Register client
	defer s.removeClient(client)

	for {
		req, err := stream.Recv()
		if err != nil {
			log.Printf("Recieve error: %v", err)
			return err
		}

		if client.Name == "" {
			if req.Info.Name != "" {
				s.setClientName(client.Uuid, req.Info.Name, req.Timestamp)
			} else {
				client.Chat.Send(&pb.Message{
					Content:   "Error: Your name is not yet set",
					Timestamp: req.Timestamp,
					Info:      &pb.ClientInfo{Uuid: client.Uuid, Name: client.Name},
				})

				continue
			}
		}

		s.broadcast(&pb.Message{
			Content:   req.Content,
			Timestamp: req.Timestamp,
			Info:      &pb.ClientInfo{Uuid: client.Uuid, Name: client.Name},
		})
	}

	return nil
}

func (s *chatServer) setClientName(id string, name string, timestamp *pb.Lamport) {
	s.lock.Lock()
	defer s.lock.Unlock()

	client := s.clients[id]
	client.Name = name

	s.broadcast(&pb.Message{
		Content:   "User joined: " + name,
		Timestamp: timestamp,
		Info: &pb.ClientInfo{
			Uuid: "11111111-1111-1111-1111-111111111111",
			Name: "Server",
		},
	})
}

func (s *chatServer) broadcast(message *pb.Message) {
	for key, ss := range s.clients {
		if err := ss.Chat.Send(message); err != nil {
			log.Printf("Could not send message for client id %s: %v", key, err)
		}
	}
}
