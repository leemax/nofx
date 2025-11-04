package manager

import (
	"nofx/database"
	"sort"
	"time"
)

// ClosedPosition 代表一个已平仓的头寸，包含其完整的盈亏信息
type ClosedPosition struct {
	Symbol        string    `json:"symbol"`
	OpenTime      time.Time `json:"open_time"`
	CloseTime     time.Time `json:"close_time"`
	Duration      string    `json:"duration"`
	TotalQuantity float64   `json:"total_quantity"`
	AvgOpenPrice  float64   `json:"avg_open_price"`
	AvgClosePrice float64   `json:"avg_close_price"`
	TotalFees     float64   `json:"total_fees"`
	NetProfit     float64   `json:"net_profit"`
}

// MatchPositions 将原始交易记录匹配成已平仓的头寸
func MatchPositions(trades []database.TradeRecord) []ClosedPosition {
	tradesBySymbol := make(map[string][]database.TradeRecord)
	for _, t := range trades {
		tradesBySymbol[t.Symbol] = append(tradesBySymbol[t.Symbol], t)
	}

	var closedPositions []ClosedPosition

	for _, symbolTrades := range tradesBySymbol {
		// 按时间升序排序
		sort.Slice(symbolTrades, func(i, j int) bool {
			return symbolTrades[i].Timestamp.Before(symbolTrades[j].Timestamp)
		})

		openBuys := []database.TradeRecord{}
		openSells := []database.TradeRecord{}

		for _, trade := range symbolTrades {
			tradeCopy := trade // 创建副本以防修改原始数据

			if tradeCopy.IsBuyer {
				// 当前是买单，尝试匹配并平掉已有的空头仓位 (openSells)
				closed, remainingSells, remainingTrade := match(&tradeCopy, openSells, false)
				closedPositions = append(closedPositions, closed...)
				openSells = remainingSells
				// 如果平掉所有空头后还有剩余，则作为新的多头开仓
				if remainingTrade.Quantity > 0 {
					openBuys = append(openBuys, remainingTrade)
				}
			} else {
				// 当前是卖单，尝试匹配并平掉已有的多头仓位 (openBuys)
				closed, remainingBuys, remainingTrade := match(&tradeCopy, openBuys, true)
				closedPositions = append(closedPositions, closed...)
				openBuys = remainingBuys
				// 如果平掉所有多头后还有剩余，则作为新的空头开仓
				if remainingTrade.Quantity > 0 {
					openSells = append(openSells, remainingTrade)
				}
			}
		}
	}

	// 按平仓时间倒序排序，最新的在前面
	sort.Slice(closedPositions, func(i, j int) bool {
		return closedPositions[i].CloseTime.After(closedPositions[j].CloseTime)
	})

	return closedPositions
}

// match 核心匹配函数
// closingTrade: 当前用于平仓的交易
// openTrades: FIFO队列，存储所有未平仓的交易
// isLongPosition: 标记我们正在平的是多头仓位 (true) 还是空头仓位 (false)
func match(closingTrade *database.TradeRecord, openTrades []database.TradeRecord, isLongPosition bool) ([]ClosedPosition, []database.TradeRecord, database.TradeRecord) {
	var closedPositions []ClosedPosition
	var remainingOpenTrades []database.TradeRecord

	closingQty := closingTrade.Quantity

	for _, openTrade := range openTrades {
		if closingQty == 0 {
			remainingOpenTrades = append(remainingOpenTrades, openTrade)
			continue
		}

		matchQty := min(closingQty, openTrade.Quantity)

		// 计算手续费占比
		openTradeFee := 0.0
		if openTrade.Quantity > 0 {
			openTradeFee = openTrade.Commission * (matchQty / openTrade.Quantity)
		}
		closingTradeFee := 0.0
		if closingTrade.Quantity > 0 {
			closingTradeFee = closingTrade.Commission * (matchQty / closingTrade.Quantity)
		}
		totalFees := openTradeFee + closingTradeFee

		var netProfit float64
		if isLongPosition { // 平多仓：卖出价 - 买入价
			netProfit = (closingTrade.Price - openTrade.Price) * matchQty - totalFees
		} else { // 平空仓：卖出价 - 买入价
			netProfit = (openTrade.Price - closingTrade.Price) * matchQty - totalFees
		}

		closed := ClosedPosition{
			Symbol:        closingTrade.Symbol,
			OpenTime:      openTrade.Timestamp,
			CloseTime:     closingTrade.Timestamp,
			Duration:      closingTrade.Timestamp.Sub(openTrade.Timestamp).String(),
			TotalQuantity: matchQty,
			AvgOpenPrice:  openTrade.Price,
			AvgClosePrice: closingTrade.Price,
			TotalFees:     totalFees,
			NetProfit:     netProfit,
		}
		closedPositions = append(closedPositions, closed)

		closingQty -= matchQty
		openTrade.Quantity -= matchQty

		if openTrade.Quantity > 0 {
			remainingOpenTrades = append(remainingOpenTrades, openTrade)
		}
	}

	closingTrade.Quantity = closingQty
	return closedPositions, remainingOpenTrades, *closingTrade
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}