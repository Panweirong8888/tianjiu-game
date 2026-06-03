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
	ID     string
	Name   string
	Conn   *websocket.Conn
	Hand   []Card
	WinCnt int // 赢的轮数，用于最后一轮判断资格
}

type GameRoom struct {
	Players        map[string]*Player
	AllCards       []Card
	CurrentRound   int
	Banker         int
	LastCards      []Card            // 本轮基准牌（首家出的牌）
	StarterID      string            // 本轮首家ID
	RequiredCount  int               // 本轮出牌数量基数
	PlayersActed   map[string]bool   // 本轮已操作的玩家
	PlayedCards    map[string][]Card // 本轮每位玩家出的牌（若弃牌则不存在）
	FoldedPlayers  map[string]bool   // 本轮弃牌的玩家
	mutex          sync.RWMutex
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

// 重置本轮状态
func (r *GameRoom) resetTrick() {
	r.LastCards = nil
	r.StarterID = ""
	r.RequiredCount = 0
	r.PlayersActed = make(map[string]bool)
	r.PlayedCards = make(map[string][]Card)
	r.FoldedPlayers = make(map[string]bool)
}

// 发牌给4个玩家，每个玩家8张 (32张牌完全分配，不重复)
func (r *GameRoom) dealCards() {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// 初始化牌库
	r.initializeDeck()

	// 洗牌
	r.shuffleDeck()

	// 清空玩家手牌并重置胜利计数
	for _, player := range r.Players {
		player.Hand = []Card{}
		player.WinCnt = 0
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

	// 重置轮状态
	r.resetTrick()

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

// 比较两名玩家在本轮的牌大小，考虑至尊特殊规则
// 返回 1 if a>b, -1 if a<b, 0 if equal
func compareForTrick(aCards []Card, aID string, bCards []Card, bID string, starterID string) int {
	comboA := getCardCombinationType(aCards)
	comboB := getCardCombinationType(bCards)

	// 类型必须相同才能比较（在游戏流里应已被强制）
	if comboA.Type != comboB.Type {
		return 0
	}

	// 至尊特殊处理：如果某方是至尊
	if comboA.Type == TYPE_SUPREME || comboB.Type == TYPE_SUPREME {
		// 如果A是至尊
		if comboA.Type == TYPE_SUPREME && comboB.Type == TYPE_SUPREME {
			// 罕见：同时为至尊，按照starter优先
			if aID == starterID && bID != starterID {
				return 1
			} else if bID == starterID && aID != starterID {
				return -1
			}
			return 0
		}
		if comboA.Type == TYPE_SUPREME {
			if aID == starterID {
				return 1
			}
			return -1
		}
		if comboB.Type == TYPE_SUPREME {
			if bID == starterID {
				return -1
			}
			return 1
		}
	}

	// 其余按 Rank 比较
	if comboA.Rank > comboB.Rank {
		return 1
	} else if comboA.Rank < comboB.Rank {
		return -1
	}
	return 0
}

func handleGameMessage(room *GameRoom, player *Player, msg GameMessage) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	switch msg.Type {
	case "move":
		log.Printf("玩家 %s 请求出牌: %v", player.Name, msg.Cards)

		// 检查玩家是否已经在本轮行动
		if room.PlayersActed == nil {
			room.PlayersActed = make(map[string]bool)
		}
		if room.PlayedCards == nil {
			room.PlayedCards = make(map[string][]Card)
		}
		if room.FoldedPlayers == nil {
			room.FoldedPlayers = make(map[string]bool)
		}

		if room.PlayersActed[player.ID] {
			// 已经行动过
			errMsg := GameMessage{Type: "invalid_move", PlayerID: player.ID, Data: map[string]interface{}{"reason": "您本轮已行动，不能重复出牌或弃牌"}}
			data, _ := json.Marshal(errMsg)
			player.Conn.WriteMessage(websocket.TextMessage, data)
			return
		}

		// 如果还没有首家出牌，本次出牌为首家
		if room.RequiredCount == 0 {
			// 首家出牌，设置本轮参数
			room.RequiredCount = len(msg.Cards)
			room.LastCards = msg.Cards
			room.StarterID = player.ID

			// 特殊规则：如果这是单张决胜轮（每位玩家手牌数均为1），只有曾获胜过的玩家可出单张
			allOne := true
			for _, p := range room.Players {
				if len(p.Hand) != 1 {
					allOne = false
					break
				}
			}
			if allOne && room.RequiredCount == 1 && player.WinCnt == 0 {
				// 不允许出单张
				errMsg := GameMessage{Type: "invalid_move", PlayerID: player.ID, Data: map[string]interface{}{"reason": "无资格出单张，需弃牌（前轮未赢过）"}}
				data, _ := json.Marshal(errMsg)
				player.Conn.WriteMessage(websocket.TextMessage, data)
				return
			}

			// 验证首家出的牌是否合法（仅验证组合合法性）
			valid, reason := isValidMove(nil, msg.Cards, player.Hand)
			if !valid {
				errMsg := GameMessage{Type: "invalid_move", PlayerID: player.ID, Data: map[string]interface{}{"reason": reason}}
				data, _ := json.Marshal(errMsg)
				player.Conn.WriteMessage(websocket.TextMessage, data)
				return
			}

			// 记录首家出的牌
			room.PlayedCards[player.ID] = msg.Cards
			room.PlayersActed[player.ID] = true
			// 移除玩家手牌
			player.Hand = removeCardsFromHand(player.Hand, msg.Cards)
		} else {
			// 非首家出牌，必须出相同数量
			if len(msg.Cards) != room.RequiredCount {
				errMsg := GameMessage{Type: "invalid_move", PlayerID: player.ID, Data: map[string]interface{}{"reason": "出牌数量必须与首家一致"}}
				data, _ := json.Marshal(errMsg)
				player.Conn.WriteMessage(websocket.TextMessage, data)
				return
			}

			// 如果这是单张决胜轮，检查资格
			if room.RequiredCount == 1 {
				allOne := true
				for _, p := range room.Players {
					if len(p.Hand) != 1 {
						allOne = false
						break
					}
				}
				if allOne && player.WinCnt == 0 {
					errMsg := GameMessage{Type: "invalid_move", PlayerID: player.ID, Data: map[string]interface{}{"reason": "无资格出单张，需弃牌（前轮未赢过）"}}
					data, _ := json.Marshal(errMsg)
					player.Conn.WriteMessage(websocket.TextMessage, data)
					return
				}
			}

			// 验证出牌是否合法（与首家比较）
			valid, reason := isValidMove(room.LastCards, msg.Cards, player.Hand)
			if !valid {
				errMsg := GameMessage{Type: "invalid_move", PlayerID: player.ID, Data: map[string]interface{}{"reason": reason}}
				data, _ := json.Marshal(errMsg)
				player.Conn.WriteMessage(websocket.TextMessage, data)
				return
			}

			// 记录玩家出牌并移除手牌
			room.PlayedCards[player.ID] = msg.Cards
			room.PlayersActed[player.ID] = true
			player.Hand = removeCardsFromHand(player.Hand, msg.Cards)
		}

		// 广播玩家出牌
		room.broadcast(GameMessage{Type: "player_move", PlayerID: player.ID, Cards: msg.Cards, Data: map[string]interface{}{"playerName": player.Name}})

		// 检查本轮是否所有玩家都已行动
		if len(room.PlayersActed) == len(room.Players) {
			// 评估本轮赢家
			winnerID := ""
			var winnerCards []Card
			// 找到首家在played order中的一项作为初始赢家
			if cards, ok := room.PlayedCards[room.StarterID]; ok {
				winnerID = room.StarterID
				winnerCards = cards
			} else {
				// 如果首家弃牌（理论上首家不应弃牌），从任意已出玩家中选一个开始
				for pid, cards := range room.PlayedCards {
					winnerID = pid
					winnerCards = cards
					break
				}
			}

			for pid, cards := range room.PlayedCards {
				if pid == winnerID {
					continue
				}
				cmp := compareForTrick(cards, pid, winnerCards, winnerID, room.StarterID)
				if cmp == 1 {
					winnerID = pid
					winnerCards = cards
				}
			}

			// 宣布赢家
			if winnerID != "" {
				winner := room.Players[winnerID]
				winner.WinCnt++
				room.broadcast(GameMessage{Type: "round_result", PlayerID: winnerID, Data: map[string]interface{}{"playerName": winner.Name}})
				log.Printf("本轮赢家: %s", winner.Name)
				// 下一轮由赢家先出
				r.resetTrick()
				room.StarterID = winnerID
			} else {
				// 所有人都弃牌？重置并下一轮由上轮starter继续先出
				r.resetTrick()
			}
		}

	case "fold":
		log.Printf("玩家 %s 请求弃牌: %v", player.Name, msg.Cards)

		if room.PlayersActed == nil {
			room.PlayersActed = make(map[string]bool)
		}
		if room.FoldedPlayers == nil {
			room.FoldedPlayers = make(map[string]bool)
		}
		if room.RequiredCount == 0 {
			// 无法在首家之前弃牌
			errMsg := GameMessage{Type: "invalid_move", PlayerID: player.ID, Data: map[string]interface{}{"reason": "本轮尚未开始，不能弃牌"}}
			data, _ := json.Marshal(errMsg)
			player.Conn.WriteMessage(websocket.TextMessage, data)
			return
		}

		if room.PlayersActed[player.ID] {
			errMsg := GameMessage{Type: "invalid_move", PlayerID: player.ID, Data: map[string]interface{}{"reason": "您本轮已行动，不能重复弃牌"}}
			data, _ := json.Marshal(errMsg)
			player.Conn.WriteMessage(websocket.TextMessage, data)
			return
		}

		// 要弃牌的数量必须等于 RequiredCount
		if len(msg.Cards) != room.RequiredCount {
			errMsg := GameMessage{Type: "invalid_move", PlayerID: player.ID, Data: map[string]interface{}{"reason": "弃牌数量必须等于本轮出牌数量"}}
			data, _ := json.Marshal(errMsg)
			player.Conn.WriteMessage(websocket.TextMessage, data)
			return
		}

		// 验证玩家手中是否有这些牌
		// 简单调用 removeCardsFromHand 并比对长度
		remaining := removeCardsFromHand(player.Hand, msg.Cards)
		if len(remaining) == len(player.Hand) {
			errMsg := GameMessage{Type: "invalid_move", PlayerID: player.ID, Data: map[string]interface{}{"reason": "弃牌中包含无效牌或数量不正确"}}
			data, _ := json.Marshal(errMsg)
			player.Conn.WriteMessage(websocket.TextMessage, data)
			return
		}

		// 确认弃牌并移除
		player.Hand = remaining
		room.PlayersActed[player.ID] = true
		room.FoldedPlayers[player.ID] = true

		// 广播弃牌
		room.broadcast(GameMessage{Type: "player_fold", PlayerID: player.ID, Data: map[string]interface{}{"playerName": player.Name}})

		// 检查本轮是否所有玩家都已行动
		if len(room.PlayersActed) == len(room.Players) {
			// 同上评估赢家
			winnerID := ""
			var winnerCards []Card
			if cards, ok := room.PlayedCards[room.StarterID]; ok {
				winnerID = room.StarterID
				winnerCards = cards
			} else {
				for pid, cards := range room.PlayedCards {
					winnerID = pid
					winnerCards = cards
					break
				}
			}

			for pid, cards := range room.PlayedCards {
				if pid == winnerID {
					continue
				}
				cmp := compareForTrick(cards, pid, winnerCards, winnerID, room.StarterID)
				if cmp == 1 {
					winnerID = pid
					winnerCards = cards
				}
			}

			if winnerID != "" {
				winner := room.Players[winnerID]
				winner.WinCnt++
				room.broadcast(GameMessage{Type: "round_result", PlayerID: winnerID, Data: map[string]interface{}{"playerName": winner.Name}})
				log.Printf("本轮赢家: %s", winner.Name)
				r.resetTrick()
				room.StarterID = winnerID
			} else {
				r.resetTrick()
			}
		}

	}
}
