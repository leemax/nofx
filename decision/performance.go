package decision

import (
	"math"
	"nofx/database"
	"nofx/logger"
)

// PerformanceAnalysis 交易表现分析
type PerformanceAnalysis struct {
	TotalTrades   int                           `json:"total_trades"`
	WinningTrades int                           `json:"winning_trades"`
	LosingTrades  int                           `json:"losing_trades"`
	WinRate       float64                       `json:"win_rate"`
	AvgWin        float64                       `json:"avg_win"`
	AvgLoss       float64                       `json:"avg_loss"`
	ProfitFactor  float64                       `json:"profit_factor"`
	SharpeRatio   float64                       `json:"sharpe_ratio"`
	SymbolStats   map[string]*SymbolPerformance `json:"symbol_stats"`
	BestSymbol    string                        `json:"best_symbol"`
	WorstSymbol   string                        `json:"worst_symbol"`
}

// SymbolPerformance 币种表现统计
type SymbolPerformance struct {
	Symbol        string  `json:"symbol"`
	TotalTrades   int     `json:"total_trades"`
	WinningTrades int     `json:"winning_trades"`
	LosingTrades  int     `json:"losing_trades"`
	WinRate       float64 `json:"win_rate"`
	TotalPnL      float64 `json:"total_pn_l"`
	AvgPnL        float64 `json:"avg_pn_l"`
}

// Analyze a trader's performance based on their closed positions.
func Analyze(traderID string, records []*logger.DecisionRecord) (*PerformanceAnalysis, error) {
	trades, err := database.GetTradesByTraderID(traderID)
	if err != nil {
		return nil, err
	}

	closedPositions := MatchPositions(trades)

	analysis := &PerformanceAnalysis{
		SymbolStats: make(map[string]*SymbolPerformance),
	}

	var totalWinAmount float64
	var totalLossAmount float64

	for _, pos := range closedPositions {
		analysis.TotalTrades++
		netProfit := pos.NetProfit

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

	bestPnL := -1e9
	worstPnL := 1e9
	for symbol, stats := range analysis.SymbolStats {
		if stats.TotalTrades > 0 {
			stats.WinRate = (float64(stats.WinningTrades) / float64(stats.TotalTrades)) * 100
			stats.AvgPnL = stats.TotalPnL / float64(stats.TotalTrades)
		}

		if stats.TotalPnL > bestPnL {
			bestPnL = stats.TotalPnL
			analysis.BestSymbol = symbol
		}
		if stats.TotalPnL < worstPnL {
			worstPnL = stats.TotalPnL
			analysis.WorstSymbol = symbol
		}
	}

	// Calculate Sharpe Ratio
	if len(records) > 1 {
		var returns []float64

		for i := 1; i < len(records); i++ {
			currentEquity := records[i].AccountState.TotalBalance
			previousEquity := records[i-1].AccountState.TotalBalance
			if previousEquity > 0 {
				returns = append(returns, (currentEquity-previousEquity)/previousEquity)
			}
		}

		if len(returns) > 0 {
			// Calculate average daily return
			var sumReturns float64
			for _, r := range returns {
				sumReturns += r
			}
			avgReturn := sumReturns / float64(len(returns))

			// Calculate standard deviation of returns
			var sumSquaredDiff float64
			for _, r := range returns {
				sumSquaredDiff += (r - avgReturn) * (r - avgReturn)
			}
			stdDev := math.Sqrt(sumSquaredDiff / float64(len(returns)))

			// Assuming a risk-free rate of 0 for simplicity in a short-term trading context
			if stdDev > 0 {
							// Annualize Sharpe Ratio (assuming 24/7 trading, 365 days * 24 hours * 60 minutes / 3 minutes per cycle = 175200 cycles/year)
							// This is a rough approximation, a more precise annualization would depend on the return frequency.
							annualizationFactor := math.Sqrt(float64(175200) / float64(len(returns)))
							analysis.SharpeRatio = (avgReturn / stdDev) * annualizationFactor			} else {
				analysis.SharpeRatio = 999.0 // Infinite Sharpe if no volatility and positive return
			}
		}
	}

	return analysis, nil
}
