package decision

import (
	"fmt"
	"nofx/database"
	"nofx/logger"
	"time"
)

// PromptPerformanceAnalysis 针对特定提示词的交易表现分析
type PromptPerformanceAnalysis struct {
	PromptName    string                        `json:"prompt_name"`
	TotalTrades   int                           `json:"total_trades"`
	WinningTrades int                           `json:"winning_trades"`
	LosingTrades  int                           `json:"losing_trades"`
	WinRate       float64                       `json:"win_rate"`
	AvgWin        float64                       `json:"avg_win"`
	AvgLoss       float64                       `json:"avg_loss"`
	ProfitFactor  float64                       `json:"profit_factor"`
	TotalPnL      float64                       `json:"total_pn_l"`
	SymbolStats   map[string]*SymbolPerformance `json:"symbol_stats"` // 沿用decision/performance.go中的SymbolPerformance
}

// AnalyzePerformanceByPrompt 根据指定的traderID和提示词名称分析交易表现
func AnalyzePerformanceByPrompt(traderID string, promptName string) (*PromptPerformanceAnalysis, error) {
	// 1. 获取指定提示词的完整内容
	var targetPromptContent string
	templates := GetAllPromptTemplates()
	for _, tpl := range templates {
		if tpl.ID == promptName {
			targetPromptContent = tpl.Content
			break
		}
	}

	if targetPromptContent == "" {
		return nil, fmt.Errorf("未找到提示词模板: %s", promptName)
	}

	// 2. 获取所有AI决策记录
	aiDecisions, err := database.GetAIDecisionsByTraderID(traderID)
	if err != nil {
		return nil, fmt.Errorf("获取AI决策记录失败: %w", err)
	}

	// 3. 过滤出与指定提示词相关的决策
	var filteredDecisions []*database.AIDecisionRecord
	for _, dec := range aiDecisions {
		// 精确匹配input_prompt内容
		if dec.InputPrompt == targetPromptContent {
			filteredDecisions = append(filteredDecisions, dec)
		}
	}

	if len(filteredDecisions) == 0 {
		return nil, fmt.Errorf("未找到与提示词 '%s' 相关的决策记录", promptName)
	}

	// 4. 获取所有交易记录
	allTrades, err := database.GetTradesByTraderID(traderID)
	if err != nil {
		return nil, fmt.Errorf("获取交易记录失败: %w", err)
	}

	// 5. 关联决策和交易，并进行性能分析
	analysis := &PromptPerformanceAnalysis{
		PromptName:  promptName,
		SymbolStats: make(map[string]*SymbolPerformance),
	}

	var totalWinAmount float64
	var totalLossAmount float64

	// 遍历过滤后的决策，找到每个决策周期内的交易
	for i, dec := range filteredDecisions {
		var startTime time.Time
		var endTime time.Time

		startTime = dec.Timestamp
		if i+1 < len(filteredDecisions) {
			endTime = filteredDecisions[i+1].Timestamp
		} else {
			// 如果是最后一个决策，结束时间设置为当前时间或一个合理的最大值
			endTime = time.Now()
		}

		// 找到在此决策周期内发生的交易
		var tradesInCycle []database.TradeRecord
		for _, trade := range allTrades {
			if trade.Timestamp.After(startTime) && trade.Timestamp.Before(endTime) {
				tradesInCycle = append(tradesInCycle, trade)
			}
		}

		// 对这些交易进行头寸匹配和盈亏计算
		closedPositions := MatchPositions(tradesInCycle)

		for _, pos := range closedPositions {
			analysis.TotalTrades++
			netProfit := pos.NetProfit
			analysis.TotalPnL += netProfit

			if netProfit > 0 {
				analysis.WinningTrades++
				totalWinAmount += netProfit
			} else if netProfit < 0 {
				analysis.LosingTrades++
				totalLossAmount += netProfit
			}

			if _, exists := analysis.SymbolStats[pos.Symbol]; !exists {
				analysis.SymbolStats[pos.Symbol] = &SymbolPerformance{Symbol: pos.Symbol}
			}
			stats := analysis.SymbolStats[pos.Symbol]
			stats.TotalTrades++
			stats.TotalPnL += netProfit
			if netProfit > 0 {
				stats.WinningTrades++
			} else if netProfit < 0 {
				stats.LosingTrades++
			}
		}
	}

	// 计算最终指标
	if analysis.TotalTrades > 0 {
		analysis.WinRate = (float64(analysis.WinningTrades) / float64(analysis.TotalTrades)) * 100
	}

	if analysis.WinningTrades > 0 {
		analysis.AvgWin = totalWinAmount / float64(analysis.WinningTrades)
	}
	if analysis.LosingTrades > 0 {
		analysis.AvgLoss = totalLossAmount / float64(analysis.LosingTrades)
	}

	if totalLossAmount != 0 {
		analysis.ProfitFactor = totalWinAmount / -totalLossAmount
	} else if totalWinAmount > 0 {
		analysis.ProfitFactor = 999.0
	}

	// 更新SymbolStats的WinRate和AvgPnL
	for _, stats := range analysis.SymbolStats {
		if stats.TotalTrades > 0 {
			stats.WinRate = (float64(stats.WinningTrades) / float64(stats.TotalTrades)) * 100
			stats.AvgPnL = stats.TotalPnL / float64(stats.TotalTrades)
		}
	}

	return analysis, nil
}
