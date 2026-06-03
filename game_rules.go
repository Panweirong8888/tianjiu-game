package main

type CardCombination struct {
	Cards     []Card
	Type      string // 组合类型（中文）
	Rank      int    // 排名 (越高越大)
	IsSupreme bool   // 是否是至尊(六公生鸡)
}

// 牌型组合类型常量（中文）
const (
	TYPE_SUPREME           = "至尊"           // 至尊 (六公生鸡)
	TYPE_WIN_BLACK         = "文黑组合"       // 文黑组合 (天九、地八、人七、鹅五)
	TYPE_DOUBLE_WIN_BLACK1 = "双文+黑"        // 双文子 + 黑子 (天天九、地地八、人人七、鹅鹅五)
	TYPE_DOUBLE_WIN_BLACK2 = "文+双黑"        // 文子 + 双黑子 (天九九、地八八、人七七、鹅五五)
	TYPE_DOUBLE_WIN        = "双文子"         // 双文子 (天天、地地等)
	TYPE_DOUBLE_BLACK      = "双黑子"         // 双黑子 (九九、八八等)
	TYPE_SINGLE            = "单张"           // 单张牌
	TYPE_FOUR              = "四张组合"       // 4张牌组合
)

// 判断是否为至尊 (六公 + 生鸡)
func isSupreme(cards []Card) bool {
	if len(cards) != 2 {
		return false
	}
	card1, card2 := cards[0], cards[1]
	return (card1.Name == "六公" && card2.Name == "生鸡") ||
		(card1.Name == "生鸡" && card2.Name == "六公")
}

// 判断是否为文黑组合 (天九、地八、人七、鹅五)
func isWinBlackCombination(cards []Card) bool {
	if len(cards) != 2 {
		return false
	}

	validCombos := map[string]string{
		"天": "九", "地": "八", "人": "七", "鹅": "五",
	}

	card1, card2 := cards[0], cards[1]

	// 确保一个是文子，一个是黑子
	if card1.Type == card2.Type {
		return false
	}

	// 检查组合
	for winName, blackName := range validCombos {
		if (card1.Name == winName && card2.Name == blackName) ||
			(card2.Name == winName && card1.Name == blackName) {
			return true
		}
	}

	return false
}

// 判断是否为双文子 (天天、地地等)
func isDoubleWin(cards []Card) bool {
	return len(cards) == 2 && cards[0].Type == "文" && cards[1].Type == "文" &&
		cards[0].Name == cards[1].Name
}

// 判断是否为双黑子 (九九、八八等)
func isDoubleBlack(cards []Card) bool {
	return len(cards) == 2 && cards[0].Type == "黑" && cards[1].Type == "黑" &&
		cards[0].Name == cards[1].Name
}

// 判断是否为3张牌的第一种类型: 双文子 + 黑子 (天天九、地地八、人人七、鹅鹅五)
func isDoubleWinWithBlack(cards []Card) bool {
	if len(cards) != 3 {
		return false
	}

	// 统计各类型的牌
	var winCards, blackCards []Card
	for _, card := range cards {
		if card.Type == "文" {
			winCards = append(winCards, card)
		} else {
			blackCards = append(blackCards, card)
		}
	}

	// 必须是2张文子 + 1张黑子
	if len(winCards) != 2 || len(blackCards) != 1 {
		return false
	}

	// 2张文子必须相同
	if winCards[0].Name != winCards[1].Name {
		return false
	}

	// 检查是否是有效的组合 (天天九、地地八、人人七、鹅鹅五)
	validCombos := map[string]string{
		"天": "九", "地": "八", "人": "七", "鹅": "五",
	}

	expectedBlack, exists := validCombos[winCards[0].Name]
	return exists && blackCards[0].Name == expectedBlack
}

// 判断是否为3张牌的第二种类型: 文子 + 双黑子 (天九九、地八八、人七七、鹅五五)
func isWinWithDoubleBlack(cards []Card) bool {
	if len(cards) != 3 {
		return false
	}

	// 统计各类型的牌
	var winCards, blackCards []Card
	for _, card := range cards {
		if card.Type == "文" {
			winCards = append(winCards, card)
		} else {
			blackCards = append(blackCards, card)
		}
	}

	// 必须是1张文子 + 2张黑子
	if len(winCards) != 1 || len(blackCards) != 2 {
		return false
	}

	// 2张黑子必须相同
	if blackCards[0].Name != blackCards[1].Name {
		return false
	}

	// 检查是否是有效的组合 (天九九、地八八、人七七、鹅五五)
	validCombos := map[string]string{
		"天": "九", "地": "八", "人": "七", "鹅": "五",
	}

	expectedBlack, exists := validCombos[winCards[0].Name]
	return exists && blackCards[0].Name == expectedBlack
}

// 获取牌组的类型和等级（中文类型）
func getCardCombinationType(cards []Card) CardCombination {
	combo := CardCombination{
		Cards: cards,
	}

	switch len(cards) {
	case 1:
		combo.Type = TYPE_SINGLE
		if cards[0].Type == "文" {
			combo.Rank = 100 + cards[0].Value
		} else {
			combo.Rank = 50 + cards[0].Value
		}

	case 2:
		if isSupreme(cards) {
			combo.Type = TYPE_SUPREME
			combo.IsSupreme = true
			combo.Rank = 1000
		} else if isWinBlackCombination(cards) {
			combo.Type = TYPE_WIN_BLACK
			// 以文子value决定大小（文子越大越大）
			var winCard Card
			if cards[0].Type == "文" { winCard = cards[0] } else { winCard = cards[1] }
			combo.Rank = 900 + winCard.Value
		} else if isDoubleWin(cards) {
			combo.Type = TYPE_DOUBLE_WIN
			combo.Rank = 800 + cards[0].Value
		} else if isDoubleBlack(cards) {
			combo.Type = TYPE_DOUBLE_BLACK
			combo.Rank = 700 + cards[0].Value
		}

	case 3:
		if isDoubleWinWithBlack(cards) {
			combo.Type = TYPE_DOUBLE_WIN_BLACK1
			// 3张组合里以文子的value决定大小（天>地>人>鹅）
			for _, c := range cards { if c.Type == "文" { combo.Rank = 600 + c.Value; break }}
		} else if isWinWithDoubleBlack(cards) {
			combo.Type = TYPE_DOUBLE_WIN_BLACK2
			for _, c := range cards { if c.Type == "文" { combo.Rank = 500 + c.Value; break }}
		}

	case 4:
		combo.Type = TYPE_FOUR
		combo.Rank = 400
	}

	return combo
}

// 比较两个牌组的大小
// isFirstMove: 是否是该轮的第一个出牌（用于至尊的判断）
// 返回: 1 表示 cards1 更大, -1 表示 cards2 更大, 0 表示不可比较或相等
func compareCards(cards1, cards2 []Card, isFirstMove bool) int {
	combo1 := getCardCombinationType(cards1)
	combo2 := getCardCombinationType(cards2)

	// 类型不同，不能比较（不同类型不能混打）
	if combo1.Type != combo2.Type {
		return 0 // 表示不能打
	}

	// 至尊特殊处理：先出最大，否则最小
	if combo1.Type == TYPE_SUPREME {
		if isFirstMove {
			return 1 // 至尊在先出时最大
		}
		return -1 // 至尊在非先出时最小
	}

	// 其他类型，直接比较等级
	if combo1.Rank > combo2.Rank {
		return 1
	} else if combo1.Rank < combo2.Rank {
		return -1
	}
	return 0
}

// 验证出牌是否有效
// lastCards: 上家出的牌 (如果为空表示首家出牌)
// playCards: 当前玩家要出的牌
// playerCards: 玩家当前的所有手牌
// 返回: (是否有效, 原因描述(中文))
func isValidMove(lastCards []Card, playCards []Card, playerCards []Card) (bool, string) {
	// 手中牌数量检查
	if len(playCards) > len(playerCards) {
		return false, "手中牌不足"
	}

	// 如果是首家出牌，任何有效组合都可以
	if len(lastCards) == 0 {
		combo := getCardCombinationType(playCards)
		if combo.Type == "" {
			return false, "无效的牌型组合"
		}
		return true, ""
	}

	// 非首家出牌
	// 1. 出牌数量必须相同
	if len(playCards) != len(lastCards) {
		return false, "出牌数量必须相同"
	}

	// 2. 牌型类型必须相同（不同类型不能混打）
	combo1 := getCardCombinationType(playCards)
	combo2 := getCardCombinationType(lastCards)
	if combo1.Type != combo2.Type {
		return false, "牌型类型不匹配，不能混打"
	}

	// 3. 必须比上家的牌大
	result := compareCards(playCards, lastCards, false)
	if result <= 0 {
		return false, "牌型太小"
	}

	return true, ""
}
