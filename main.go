package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"github.com/mitchellh/mapstructure"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type Broadcaster struct {
	UsersChannel chan interface{}
}

type Connector struct {
	WsConn *websocket.Conn
	Role   string
	Cid    string
}

type Server struct {
	Connectors []*Connector
}

type ReqBody struct {
	Event string      `form:"event"`
	Data  interface{} `form:"data"`
}

type RespBody struct {
	Event string      `form:"event"`
	Data  interface{} `form:"data"`

}

var upgrade = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var server = &Server{
	Connectors: make([]*Connector, 0),
}

var broadcaster = &Broadcaster{
	UsersChannel: make(chan interface{}, 0),
}



type Player struct{
	Guid string
}

func main() {
	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})
	r.GET("/ws", func(c *gin.Context) {
		ws, err := upgrade.Upgrade(c.Writer, c.Request, nil)
		role := c.Query("role")
		cid := c.Query("cid")
		conn := &Connector{
			WsConn: ws,
			Role:   role,
			Cid:    cid,
		}
		server.Connectors = append(server.Connectors, conn)
		go broadcaster.Start()
		if err != nil {
			log.Fatalln(err)
		}
		defer ws.Close()
		go func() {
			<-c.Done()
			fmt.Println("ws lost connection")
		}()
		for {
			_, data, err := ws.ReadMessage()
			body := &ReqBody{}
			if err := json.Unmarshal(data, body); err != nil {
				log.Panicln("非法格式:", err)
			}

			if err != nil {
				fmt.Println("read error")
				fmt.Println(err)
				break
			}


			player :=&Player{}

			mapstructure.Decode(body.Data,player)

			fmt.Println(player)


			broadcaster.UsersChannel <- data

		}
	})
	r.Run(":3010")
}

func (b *Broadcaster) Start() {

	for {
		select {
		case resp := <-b.UsersChannel:
			for _, user := range server.Connectors {
				fmt.Println(resp,user)
				// body := &RespBody{}

				// if err := json.Unmarshal(resp, body); err != nil {
				// 	log.Panicln("非法格式:", err)
				// }
				


				// user.WsConn.WriteMessage(1, []byte(cv))
			}
		}

	}

}
