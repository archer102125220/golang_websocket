package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/mitchellh/mapstructure"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
)

type Broadcaster struct {
	UsersChannel              chan interface{}
	StampNotifyChannel        chan *StampNotify
	StampSuccessNotifyChannel chan *StampNotifySuccess
}

type Connector struct {
	WsConn *websocket.Conn
	Role   string
	Cid    string
}

type Server struct {
	Connectors []*Connector
	Mutex      sync.Mutex  // 新增鎖
}

type ReqBody struct {
	Event string      `form:"event"`
	Data  interface{} `form:"data"`
}

type RespBody struct {
	Event string      `form:"event" json:"event"`
	Data  interface{} `form:"data" json:"data"`
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
	UsersChannel:              make(chan interface{}, 0),
	StampNotifyChannel:        make(chan *StampNotify, 0),
	StampSuccessNotifyChannel: make(chan *StampNotifySuccess, 0),
}

type StampNotify struct {
	Guid           string `mapstructure:"guid" json:"guid"`
	Uid            string `mapstructure:"uid" json:"uid"`
	Desc           string `mapstructure:"desc" json:"desc"`
	PrizeID        int    `mapstructure:"prize_id" json:"prize_id"`
	ItemName       string `mapstructure:"item_name" json:"item_name"`
	ExchangeNum    int    `mapstructure:"exchange_num" json:"exchange_num"`
	SpendStampNum  int    `mapstructure:"spend_stamp_num" json:"spend_stamp_num"`
	RemainStampNum int    `mapstructure:"remain_stamp_num" json:"remain_stamp_num"`
}

type StampNotifySuccess struct {
	Uid     string `mapstructure:"uid" json:"uid"`
	Code    string `mapstructure:"code" json:"code"`
	Message string `mapstructure:"message" json:"message"`
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
			removeConnector(conn)  // 移除連線
		}()
		for {
			_, data, err := ws.ReadMessage()
			body := &ReqBody{}
			if err := json.Unmarshal(data, body); err != nil {
				fmt.Println(err)
				break
			}

			if err != nil {
				fmt.Println("read error")
				fmt.Println(err)
				break
			}

			event := body.Event

			switch event {
			case "SEND_STAMP_TO_CUSTOMER_NOTIFY":
				customer := &StampNotify{}
				mapstructure.Decode(body.Data, customer)
				broadcaster.StampNotifyChannel <- customer
			case "SEND_STAMP_SUCCESS_NOTIFY":
				success := &StampNotifySuccess{}
				mapstructure.Decode(body.Data, success)
				broadcaster.StampSuccessNotifyChannel <- success
			}

		}
	})

	err2 := godotenv.Load()
	if err2 != nil {
		log.Fatal("Error loading .env file")
	}

	certPem := os.Getenv("DOMAIN_CERT_PEM")
	privatePem := os.Getenv("DOMAIN_PRIVATE_PEM")
	if os.Getenv("ENVIRONMENT") == "local" {
		err := r.Run(":3000")
		if err != nil {
			fmt.Println("Error starting server:", err)
		}
	} else {
		err := r.RunTLS(":3000", certPem, privatePem)
		if err != nil {
			fmt.Println("Error starting server:", err)
		}

	}

}

// 新增函數來移除連線
func removeConnector(conn *Connector) {
    server.Mutex.Lock()
    defer server.Mutex.Unlock()
    for i, c := range server.Connectors {
        if c == conn {
            server.Connectors = append(server.Connectors[:i], server.Connectors[i+1:]...)
            break
        }
    }
}

func (b *Broadcaster) Start() {

	for {
		select {
		case notify := <-b.StampNotifyChannel:
			for _, user := range server.Connectors {
				if notify.Guid == user.Cid {

					res := RespBody{
						Event: "SEND_STAMP_TO_CUSTOMER_NOTIFY",
						Data:  notify,
					}
					json, err := json.Marshal(res)
					if err != nil {
						fmt.Println(err)
						return
					}
					user.WsConn.WriteMessage(1, []byte(json))
				}
			}
		case notify := <-b.StampSuccessNotifyChannel:

			for _, user := range server.Connectors {
				if notify.Uid == user.Cid {

					res := RespBody{
						Event: "SEND_STAMP_SUCCESS_NOTIFY",
						Data:  notify,
					}
					json, err := json.Marshal(res)
					if err != nil {
						fmt.Println(err)
						return
					}
					user.WsConn.WriteMessage(1, []byte(json))
				}
			}

		}

	}

}
