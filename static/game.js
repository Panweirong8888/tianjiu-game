class TianJiuGame {
    constructor() {
        this.ws = null;
        this.playerID = this.generatePlayerID();
        this.playerName = `玩家${Math.floor(Math.random() * 10000)}`;
        this.myCards = [];
        this.selectedCards = [];
        this.otherPlayersCards = [0, 0, 0]; // 其他三个玩家的手牌数
        this.currentRound = 1;
        this.gameLog = [];
        this.cardCanvases = {
            my: document.getElementById('myCardsCanvas'),
            top: document.getElementById('topPlayerCanvas'),
            left: document.getElementById('leftPlayerCanvas'),
            right: document.getElementById('rightPlayerCanvas')
        };
        this.initializeEventListeners();
        this.initializeGame();
    }

    generatePlayerID() {
        return `player_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
    }

    initializeEventListeners() {
        document.getElementById('playBtn').addEventListener('click', () => this.playCards());
        document.getElementById('foldBtn').addEventListener('click', () => this.foldCards());
        document.getElementById('clearBtn').addEventListener('click', () => this.clearSelection());
        document.getElementById('myCardsCanvas').addEventListener('click', (e) => this.handleCardClick(e));
    }

    initializeGame() {
        document.getElementById('playerName').textContent = `玩家: ${this.playerName}`;
        this.connectWebSocket();
    }

    connectWebSocket() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        this.ws = new WebSocket(
            `${protocol}//${window.location.host}/ws?id=${this.playerID}&name=${encodeURIComponent(this.playerName)}`
        );

        this.ws.onopen = () => {
            this.addLog('已连接到游戏服务器');
            document.getElementById('gameStatus').textContent = '状态: 已连接';
        };

        this.ws.onmessage = (event) => {
            const message = JSON.parse(event.data);
            this.handleGameMessage(message);
        };

        this.ws.onerror = (error) => {
            this.addLog('连接错误: ' + error);
            document.getElementById('gameStatus').textContent = '状态: 错误';
        };

        this.ws.onclose = () => {
            this.addLog('已断开连接');
            document.getElementById('gameStatus').textContent = '状态: 已断开';
        };
    }

    sortCards() {
        this.myCards.sort((a, b) => {
            // 先按类型排序 (文在前)
            if (a.type !== b.type) {
                return a.type === '文' ? -1 : 1;
            }
            // 再按大小排序 (从大到小)
            return b.value - a.value;
        });
    }

    handleCardClick(e) {
        const canvas = e.target;
        const rect = canvas.getBoundingClientRect();
        const x = e.clientX - rect.left;
        const y = e.clientY - rect.top;

        const cardWidth = 60;
        const cardHeight = 90;
        const spacing = 70;
        const startX = (canvas.width - (this.myCards.length * spacing - 10)) / 2;
        const startY = (canvas.height - cardHeight) / 2;

        for (let i = 0; i < this.myCards.length; i++) {
            const cardX = startX + i * spacing;
            const cardY = startY;

            if (x >= cardX && x <= cardX + cardWidth &&
                y >= cardY && y <= cardY + cardHeight) {
                this.toggleCardSelection(i);
                this.renderMyCards();
                break;
            }
        }
    }

    toggleCardSelection(index) {
        const selectedIndex = this.selectedCards.indexOf(index);
        if (selectedIndex > -1) {
            this.selectedCards.splice(selectedIndex, 1);
        } else {
            this.selectedCards.push(index);
        }
        this.selectedCards.sort((a, b) => a - b);
    }

    renderMyCards() {
        const canvas = this.cardCanvases.my;
        const ctx = canvas.getContext('2d');
        ctx.clearRect(0, 0, canvas.width, canvas.height);

        const cardWidth = 60;
        const cardHeight = 90;
        const spacing = 70;
        const startX = (canvas.width - (this.myCards.length * spacing - 10)) / 2;
        const startY = (canvas.height - cardHeight) / 2;

        this.myCards.forEach((card, i) => {
            const x = startX + i * spacing;
            const isSelected = this.selectedCards.includes(i);
            const offsetY = isSelected ? -20 : 0;

            // 绘制卡牌
            ctx.fillStyle = isSelected ? '#667eea' : '#ffffff';
            ctx.strokeStyle = isSelected ? '#667eea' : '#333333';
            ctx.lineWidth = 2;
            ctx.fillRect(x, startY + offsetY, cardWidth, cardHeight);
            ctx.strokeRect(x, startY + offsetY, cardWidth, cardHeight);

            // 绘制卡牌内容
            ctx.fillStyle = isSelected ? '#ffffff' : '#333333';
            ctx.font = 'bold 14px Arial';
            ctx.textAlign = 'center';
            ctx.textBaseline = 'middle';
            ctx.fillText(card.name, x + cardWidth / 2, startY + cardHeight / 2 - 10 + offsetY);
            ctx.font = '12px Arial';
            ctx.fillText(card.type, x + cardWidth / 2, startY + cardHeight / 2 + 10 + offsetY);
        });
    }

    clearSelection() {
        this.selectedCards = [];
        this.renderMyCards();
    }

    playCards() {
        if (this.selectedCards.length === 0) {
            this.addLog('请先选择要出的牌');
            return;
        }

        const cards = this.selectedCards.map(i => this.myCards[i]);
        const message = {
            type: 'move',
            playerId: this.playerID,
            cards: cards
        };

        this.ws.send(JSON.stringify(message));
        this.addLog(`你出了 ${cards.map(c => c.name).join(' ')}`);

        // 移除出过的牌（本地乐观更新）
        this.myCards = this.myCards.filter((_, i) => !this.selectedCards.includes(i));
        this.selectedCards = [];
        this.renderMyCards();
    }

    foldCards() {
        if (this.selectedCards.length === 0) {
            this.addLog('请先选择要弃掉的牌，再点击弃牌');
            alert('请先选择要弃掉的牌，再点击弃牌');
            return;
        }

        const cards = this.selectedCards.map(i => this.myCards[i]);
        const message = {
            type: 'fold',
            playerId: this.playerID,
            cards: cards
        };

        this.ws.send(JSON.stringify(message));
        this.addLog(`你弃掉了 ${cards.map(c => c.name).join(' ')}`);

        // 移除弃掉的牌（本地乐观更新）
        this.myCards = this.myCards.filter((_, i) => !this.selectedCards.includes(i));
        this.selectedCards = [];
        this.renderMyCards();
    }

    handleGameMessage(message) {
        switch (message.type) {
            case 'deal_cards':
                // 从服务器接收发牌信息
                this.myCards = message.cards || [];
                this.sortCards();
                this.renderMyCards();
                this.addLog(`游戏开始！服务器为你分配了 ${this.myCards.length} 张牌`);
                break;
            case 'invalid_move':
                {
                    const reason = message.data && message.data.reason ? message.data.reason : '出牌无效';
                    this.addLog(`出牌无效：${reason}`);
                    alert(`出牌无效：${reason}`);
                    this.flashInvalidSelection();
                }
                break;
            case 'round_start':
                this.currentRound = message.round;
                this.addLog(`第 ${message.round} 轮开始`);
                break;
            case 'player_move':
                this.addLog(`${message.data.playerName} 出了 ${message.cards.map(c => c.name).join(' ')}`);
                break;
            case 'player_fold':
                this.addLog(`${message.data.playerName} 弃牌`);
                break;
            case 'round_result':
                this.addLog(`${message.data.playerName} 赢得了这一轮`);
                break;
            case 'player_win':
                this.addLog(`${message.data.playerName} 获胜！`);
                alert(`${message.data.playerName} 获胜！`);
                break;
            default:
                this.addLog(`收到消息：${JSON.stringify(message)}`);
                break;
        }
    }

    // 闪烁当前被选中的牌（用于提示出牌/弃牌无效）
    flashInvalidSelection() {
        if (!this.selectedCards || this.selectedCards.length === 0) return;
        const original = [...this.selectedCards];
        let flashes = 0;
        const maxFlashes = 6;
        const timer = setInterval(() => {
            if (flashes % 2 === 0) {
                this.selectedCards = []; // 隐藏选中样式
            } else {
                this.selectedCards = [...original]; // 恢复选中
            }
            this.renderMyCards();
            flashes++;
            if (flashes >= maxFlashes) {
                clearInterval(timer);
                this.selectedCards = [...original];
                this.renderMyCards();
            }
        }, 160);
    }

    renderOtherPlayersCards() {
        // 绘制其他玩家的卡牌背面
        const positions = ['top', 'left', 'right'];
        positions.forEach((pos, idx) => {
            const canvas = this.cardCanvases[pos];
            const ctx = canvas.getContext('2d');
            ctx.clearRect(0, 0, canvas.width, canvas.height);

            const cardCount = this.otherPlayersCards[idx];
            const cardWidth = 40;
            const cardHeight = 60;
            const spacing = 45;
            const totalWidth = cardCount * spacing - 5;
            const startX = (canvas.width - totalWidth) / 2;
            const startY = (canvas.height - cardHeight) / 2;

            for (let i = 0; i < cardCount; i++) {
                const x = startX + i * spacing;
                ctx.fillStyle = '#e8e8e8';
                ctx.strokeStyle = '#999';
                ctx.lineWidth = 1;
                ctx.fillRect(x, startY, cardWidth, cardHeight);
                ctx.strokeRect(x, startY, cardWidth, cardHeight);

                // 卡牌背面图案
                ctx.fillStyle = '#ccc';
                ctx.font = '10px Arial';
                ctx.textAlign = 'center';
                ctx.textBaseline = 'middle';
                ctx.fillText('天九', x + cardWidth / 2, startY + cardHeight / 2);
            }
        });
    }

    addLog(message) {
        const timestamp = new Date().toLocaleTimeString('zh-CN');
        this.gameLog.push(`[${timestamp}] ${message}`);
        const logContent = document.getElementById('gameLog');
        logContent.innerHTML = this.gameLog.slice(-10).map(log => `<div class="log-item">${log}</div>`).join('');
        logContent.scrollTop = logContent.scrollHeight;
    }
}

// 初始化游戏
window.addEventListener('DOMContentLoaded', () => {
    new TianJiuGame();
});
