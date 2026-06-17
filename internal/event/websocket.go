package event

import (
	"encoding/json"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/jasonlvhit/gocron"
	"github.com/rs/zerolog/log"
)

var upgrader = websocket.Upgrader{}

type WebsocketManager struct {
	connections map[*websocket.Conn]bool
	lock        sync.Mutex
}

type WebsocketOptions struct {
	ApiBaseUrl string
	Engine     *gin.Engine
}

var websocketManager *WebsocketManager

func RegisterWebsocketManager(options *WebsocketOptions) error {
	websocketManager = &WebsocketManager{
		connections: make(map[*websocket.Conn]bool),
		lock:        sync.Mutex{},
	}

	wsHandler := func(c *gin.Context) {
		websocketConnection, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Error().Err(err).Msg("error upgrading websocket connection")
			return
		}
		websocketManager.addConnection(websocketConnection)
	}

	options.Engine.GET(options.ApiBaseUrl+"/ws", wsHandler)

	go func() {
		websocketScheduler := gocron.NewScheduler()
		websocketScheduler.Every(10).Second().Do(func() {
			// log.Trace().Msg("Sending websocket ping")
			BroadcastWebsocketMessage(&WebsocketMessage[string]{
				Object: EventObjectPing,
				Action: EventActionPing,
				Data:   "ping",
			})
		})
		<-websocketScheduler.Start()
	}()

	return nil
}

func (w *WebsocketManager) addConnection(conn *websocket.Conn) {
	log.Debug().Msg("adding websocket connection")
	w.lock.Lock()
	w.connections[conn] = true
	w.lock.Unlock()
}

func (w *WebsocketManager) removeConnection(conn *websocket.Conn) {
	log.Debug().Msg("removing websocket connection")
	err := conn.Close()
	if err != nil {
		log.Error().Err(err).Msg("error closing websocket connection")
	}
	delete(w.connections, conn)
}

type WebsocketMessage[T any] struct {
	Object EventObject `json:"object"`
	Action EventAction `json:"action"`
	Data   T           `json:"data"`
}

func BroadcastWebsocketMessage[T any](message *WebsocketMessage[T]) error {
	jsonData, err := json.Marshal(message)
	if err != nil {
		log.Error().Err(err).Msg("error marshalling websocket message to json")
		return err
	}

	websocketManager.lock.Lock()
	badConnections := make([]*websocket.Conn, 0)
	for conn := range websocketManager.connections {
		err := conn.WriteMessage(websocket.TextMessage, jsonData)
		if err != nil {
			badConnections = append(badConnections, conn)
		}
	}

	for _, conn := range badConnections {
		websocketManager.removeConnection(conn)
	}
	websocketManager.lock.Unlock()

	return nil
}
