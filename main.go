package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Card struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value int    `json:"value"`
}

type Player struct {
	ID   string
	Name string
	Conn *websocket.Conn
	Hand []Card
}

type GameRoom struct {
	Players      map[string]*Player
	AllCards     []Card
	CurrentRound int
	Banker       int
	LastCards    []Card // 上一位玩家出的牌
	LastPlayerID string
	mutex        sync.RWMutex
}

type GameMessage struct {
	Type     string      `json:"type"`
	PlayerID string      `json:"playerId"`
	Cards    []Card      `json:"cards"`
	Data     interface{} `json:"data"`
}

var (
	rooms = make(map[string]*GameRoom)
	mu    sync.RWMutex
)

func main() {
	rand.Seed(time.Now().UnixNano())

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
	playerName := r.URL.Query().Get("name")

	player := &Player{
		ID:   playerID,
		Name: playerName,
		Conn: conn,
	}

	// 将玩家加入房间 (简单实现：所有玩家在同一个房间)
	room := getOrCreateRoom("main")
	room.addPlayer(player)

	// 当有4个玩家时，开始发牌
	if len(room.Players) == 4 {
		room.dealCards()
		room.broadcastDealCards()
	}

	for {
		var msg GameMessage
		err := conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket错误: %v", err)
			}
			room.removePlayer(playerID)
			break
		}

		handleGameMessage(room, player, msg)
	}
}

func getOrCreateRoom(roomID string) *GameRoom {
	mu.Lock()
	defer mu.Unlock()

	if room, exists := rooms[roomID]; exists {
		return room
	}

	room := &GameRoom{
		Players:      make(map[string]*Player),
		CurrentRound: 1,
		Banker:       0,
	}
	rooms[roomID] = room
	return room
}

func (r *GameRoom) addPlayer(player *Player) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.Players[player.ID] = player
	log.Printf("玩家 %s 加入游戏，当前玩家数: %d", player.Name, len(r.Players))
}

func (r *GameRoom) removePlayer(playerID string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	delete(r.Players, playerID)
	log.Printf("玩家离开游戏，当前玩家数: %d", len(r.Players))
}

// 初始化32张牌
func (r *GameRoom) initializeDeck() {
	r.AllCards = []Card{}

	// 文子 (11种，每种2张 = 22张)
	winPatterns := []struct {
		name  string
		value int
	}{
		{"天", 11}, {"地", 10}, {"人", 9}, {"鹅", 8},
		{"梅", 7}, {"长三", 6}, {"板凳", 5}, {"斧头", 4},
		{"红头十", 3}, {"高脚七", 2}, {"铃铛六", 1},
	}

	for _, pattern := range winPatterns {
		r.AllCards = append(r.AllCards, Card{Name: pattern.name, Type: "文", Value: pattern.value})
		r.AllCards = append(r.AllCards, Card{Name: pattern.name, Type: "文", Value: pattern.value})
	}

	// 黑子 (九2张、八2张、七2张、六公1张、五2张、生鸡1张 = 10张)
	blackCards := []Card{
		{Name: "九", Type: "黑", Value: 5},
		{Name: "九", Type: "黑", Value: 5},
		{Name: "八", Type: "黑", Value: 4},
		{Name: "八", Type: "黑", Value: 4},
		{Name: "七", Type: "黑", Value: 3},
		{Name: "七", Type: "黑", Value: 3},
		{Name: "六公", Type: "黑", Value: 2},
		{Name: "五", Type: "黑", Value: 1},
		{Name: "五", Type: "黑", Value: 1},
		{Name: "生鸡", Type: "黑", Value: 0},
	}

	r.AllCards = append(r.AllCards, blackCards...)
}

// 洗牌
func (r *GameRoom) shuffleDeck() {
	for i := len(r.AllCards) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		r.AllCards[i], r.AllCards[j] = r.AllCards[j], r.AllCards[i]
	}
}

// 发牌给4个玩家，每个玩家8张 (32张牌完全分配，不重复)
func (r *GameRoom) dealCards() {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// 初始化牌库
	r.initializeDeck()

	// 洗牌
	r.shuffleDeck()

	// 清空玩家手牌
	for _, player := range r.Players {
		player.Hand = []Card{}
	}

	// 将玩家转换为有序列表 (保证发牌顺序一致)
	playerList := make([]*Player, 0, len(r.Players))
	for _, p := range r.Players {
		playerList = append(playerList, p)
	}

	// 发牌：4个玩家轮流抽牌，每个玩家8张（总共32张）
	cardIndex := 0
	for i := 0; i < 8; i++ { // 每个玩家8张牌
		for j := 0; j < 4; j++ { // 4个玩家
			if cardIndex < len(r.AllCards) {
				playerList[j].Hand = append(playerList[j].Hand, r.AllCards[cardIndex])
				cardIndex++
			}
		}
	}

	// 初始化上一轮牌为空
	r.LastCards = nil
	r.LastPlayerID = ""

	log.Printf("发牌完成！共发出 %d 张牌", cardIndex)
	for i, p := range playerList {
		log.Printf("玩家 %d (%s) 获得 %d 张牌", i+1, p.Name, len(p.Hand))
	}
}

// 广播发牌信息给所有玩家
func (r *GameRoom) broadcastDealCards() {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	for _, player := range r.Players {
		msg := GameMessage{
			Type:     "deal_cards",
			PlayerID: player.ID,
			Cards:    player.Hand,
		}
		data, err := json.Marshal(msg)
		if err != nil {
			log.Println("序列化错误:", err)
			continue
		}
		err = player.Conn.WriteMessage(websocket.TextMessage, data)
		if err != nil {
			log.Println("发送错误:", err)
		}
	}
}

// 广播消息给所有玩家
func (r *GameRoom) broadcast(msg GameMessage) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	for _, player := range r.Players {
		data, err := json.Marshal(msg)
		if err != nil {
			log.Println("序列化错误:", err)
			continue
		}
		player.Conn.WriteMessage(websocket.TextMessage, data)
	}
}

// 从玩家手牌中移除已出的牌（按名称/类型/value匹配，处理重复）
func removeCardsFromHand(hand []Card, played []Card) []Card {
	remaining := make([]Card, 0, len(hand))
	used := make([]bool, len(hand))

	for _, p := range played {
		// 找到第一张匹配且尚未被使用的牌
		found := false
		for i, h := range hand {
			if used[i] {
				continue
			}
			if h.Name == p.Name && h.Type == p.Type && h.Value == p.Value {
				used[i] = true
				found = true
				break
			}
		}
		// 如果没有找到完全相同的牌，尝试按 Name+Type 匹配（不严格匹配 value）
		if !found {
			for i, h := range hand {
				if used[i] {
					continue
				}
				if h.Name == p.Name && h.Type == p.Type {
					used[i] = true
					found = true
					break
				}
			}
		}
	}

	for i, h := range hand {
		if !used[i] {
			remaining = append(remaining, h)
		}
	}
	return remaining
}

func handleGameMessage(room *GameRoom, player *Player, msg GameMessage) {
	switch msg.Type {
	case "move":
		log.Printf("玩家 %s 请求出牌: %v", player.Name, msg.Cards)
		// 校验出牌是否合法，使用 game_rules.go 中的规则
		valid, reason := isValidMove(room.LastCards, msg.Cards, player.Hand)
		if !valid {
			// 发送无效提示给当前玩家
			errMsg := GameMessage{
				Type:     "invalid_move",
				PlayerID: player.ID,
				Data:     map[string]interface{}{"reason": reason},
			}
			data, _ := json.Marshal(errMsg)
			player.Conn.WriteMessage(websocket.TextMessage, data)
			log.Printf("玩家 %s 出牌无效: %s", player.Name, reason)
			return
		}

		// 移除玩家手牌
		player.Hand = removeCardsFromHand(player.Hand, msg.Cards)

		// 更新上一手牌
		room.LastCards = msg.Cards
		room.LastPlayerID = player.ID

		// 广播玩家出牌
		room.broadcast(GameMessage{
			Type:     "player_move",
			PlayerID: player.ID,
			Cards:    msg.Cards,
			Data: map[string]interface{}{
				"playerName": player.Name,
			},
		})

		// 如果玩家手牌为空，宣布胜利（简单处理）
		if len(player.Hand) == 0 {
			room.broadcast(GameMessage{
				Type:     "player_win",
				PlayerID: player.ID,
				Data: map[string]interface{}{"playerName": player.Name},
			})
		}

	case "fold":
		log.Printf("玩家 %s 弃牌", player.Name)
		room.broadcast(GameMessage{
			Type:     "player_fold",
			PlayerID: player.ID,
			Data: map[string]interface{}{
				"playerName": player.Name,
			},
		})
	}
}
