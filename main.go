package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Game struct {
	players      []*Player
	cards        []Card
	currentRound int
	banker       int // 庄家索引
	mutex        sync.RWMutex
}

type Player struct {
	id    string
	name  string
	hand  []Card
	conn  *websocket.Conn
	mutex sync.RWMutex
}

type Card struct {
	Name  string // 如: "天", "地", "人", etc.
	Type  string // "文" 或 "黑"
	Value int    // 牌的数值用于比较
}

type GameMessage struct {
	Type    string      `json:"type"`    // "move", "fold", "start", etc.
	PlayerID string     `json:"playerId"`
	Cards   []Card      `json:"cards"`
	Data    interface{} `json:"data"`
}

func main() {
	http.HandleFunc("/", serveIndex)
	http.HandleFunc("/ws", handleWebSocket)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	fmt.Println("天九游戏服务器启动在 http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "index.html")
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket升级失败:", err)
		return
	}
	defer conn.Close()

	playerID := r.URL.Query().Get("id")
	player := &Player{
		id:   playerID,
		name: r.URL.Query().Get("name"),
		conn: conn,
	}

	for {
		var msg GameMessage
		err := conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket错误: %v", err)
			}
			break
		}

		handleGameMessage(player, msg)
	}
}

func handleGameMessage(player *Player, msg GameMessage) {
	switch msg.Type {
	case "start":
		log.Printf("玩家 %s 开始游戏", player.name)
	case "move":
		log.Printf("玩家 %s 出牌: %v", player.name, msg.Cards)
	case "fold":
		log.Printf("玩家 %s 弃牌", player.name)
	}
}

func initializeDeck() []Card {
	deck := []Card{}

	// 文子
	winPatterns := map[string]int{
		"天": 12, "地": 11, "人": 10, "鹅": 9,
		"梅": 8, "长三": 7, "板凳": 6, "斧头": 5,
		"红头十": 4, "高脚七": 3, "铃铛六": 2,
	}

	for pattern, value := range winPatterns {
		deck = append(deck, Card{Name: pattern, Type: "文", Value: value})
		deck = append(deck, Card{Name: pattern, Type: "文", Value: value})
	}

	// 黑子
	blackPatterns := map[string]int{
		"九": 5, "八": 4, "七": 3, "六公": 2, "五": 1, "生鸡": 0,
	}

	for pattern, value := range blackPatterns {
		if pattern == "六公" || pattern == "生鸡" {
			deck = append(deck, Card{Name: pattern, Type: "黑", Value: value})
		} else {
			deck = append(deck, Card{Name: pattern, Type: "黑", Value: value})
			deck = append(deck, Card{Name: pattern, Type: "黑", Value: value})
		}
	}

	return deck
}
