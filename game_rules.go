package main

type CardCombination struct {
	Cards     []Card
	Type      string // 组合类型
	Rank      int    // 排名 (越高越大)
	IsSupreme bool   // 是否是至尊(六公生鸡)
}

// 判断是否为文黑组合 (天九、地八、人七、鹅五)
func isWinBlackCombination(cards []Card) bool {
	if len(cards) != 2 {
		return false
	}

	var hasWin, hasBlack bool
	for _, card := range cards {
		if card.Type == "文" {
			hasWin = true
		} else if card.Type == "黑" {
			hasBlack = true
		}
	}

	if !hasWin || !hasBlack {
		return false
	}

	// 检查是否为有效组合: 天九、地八、人七、鹅五
	validCombos := map[string]string{
		"天": "九", "地": "八", "人": "七", "鹅": "五",
	}

	winCard := cards[0]
	blackCard := cards[1]

	if winCard.Type == "黑" {
		winCard, blackCard = blackCard, winCard
	}

	expectedBlack, exists := validCombos[winCard.Name]
	return exists && blackCard.Name == expectedBlack
}

// 判断是否为至尊 (六公生鸡)
func isSupreme(cards []Card) bool {
	if len(cards) != 2 {
		return false
	}
	card1, card2 := cards[0], cards[1]
	return (card1.Name == "六公" && card2.Name == "生鸡") ||
		(card1.Name == "生鸡" && card2.Name == "六公")
}

// 判断是否为双文子
func isDoubleWin(cards []Card) bool {
	return len(cards) == 2 && cards[0].Type == "文" && cards[1].Type == "文" &&
		cards[0].Name == cards[1].Name
}

// 判断是否为双黑子
func isDoubleBlack(cards []Card) bool {
	return len(cards) == 2 && cards[0].Type == "黑" && cards[1].Type == "黑" &&
		cards[0].Name == cards[1].Name
}

// 判断是否为3张牌的组合
func isThreeCardCombination(cards []Card) bool {
	return len(cards) == 3
}

// 判断是否为4张牌的组合
func isFourCardCombination(cards []Card) bool {
	return len(cards) == 4
}

// 比较���个组合的大小
func compareCards(cards1, cards2 []Card, isFirstMove bool) int {
	rank1 := getCardRank(cards1, isFirstMove)
	rank2 := getCardRank(cards2, isFirstMove)

	if rank1 > rank2 {
		return 1 // cards1 更大
	} else if rank1 < rank2 {
		return -1 // cards2 更大
	}
	return 0 // 相同大小
}

// 获取牌组的排名
func getCardRank(cards []Card, isFirstMove bool) int {
	switch len(cards) {
	case 1:
		return getSingleCardRank(cards[0])
	case 2:
		if isSupreme(cards) {
			if isFirstMove {
				return 1000 // 至尊在先出时最大
			}
			return 0 // 至尊在非先出时最小
		}
		if isWinBlackCombination(cards) {
			return 800 + cards[0].Value
		}
		if isDoubleWin(cards) {
			return 600 + cards[0].Value
		}
		if isDoubleBlack(cards) {
			return 400 + cards[0].Value
		}
	case 3:
		return 300 // 3张牌的组合
	case 4:
		return 200 // 4张牌的组合
	}
	return 0
}

// 获取单张牌的排名
func getSingleCardRank(card Card) int {
	if card.Type == "文" {
		return 100 + card.Value
	}
	return 50 + card.Value
}
