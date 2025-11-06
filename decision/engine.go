package decision

import (
	"encoding/json"
	"fmt"
	"log"
	"nofx/market"
	"nofx/mcp"
	"nofx/pool"
	"strings"
	"time"
)

// PositionInfo 持仓信息
type PositionInfo struct {
	Symbol           string  `json:"symbol"`
	Side             string  `json:"side"` // "long" or "short"
	EntryPrice       float64 `json:"entry_price"`
	MarkPrice        float64 `json:"mark_price"`
	Quantity         float64 `json:"quantity"`
	Leverage         int     `json:"leverage"`
	UnrealizedPnL    float64 `json:"unrealized_pnl"`
	UnrealizedPnLPct float64 `json:"unrealized_pnl_pct"`
	LiquidationPrice float64 `json:"liquidation_price"`
	MarginUsed       float64 `json:"margin_used"`
	UpdateTime       int64   `json:"update_time"` // 持仓更新时间戳（毫秒）
	IsExternal       bool    `json:"-"`           // 是否为外部仓位（不序列化）
}

// AccountInfo 账户信息
type AccountInfo struct {
	TotalEquity      float64 `json:"total_equity"`      // 账户净值
	AvailableBalance float64 `json:"available_balance"` // 可用余额
	TotalPnL         float64 `json:"total_pnl"`         // 总盈亏
	TotalPnLPct      float64 `json:"total_pnl_pct"`     // 总盈亏百分比
	MarginUsed       float64 `json:"margin_used"`       // 已用保证金
	MarginUsedPct    float64 `json:"margin_used_pct"`   // 保证金使用率
	PositionCount    int     `json:"position_count"`    // 持仓数量
}

// CandidateCoin 候选币种（来自币种池）
type CandidateCoin struct {
	Symbol  string   `json:"symbol"`
	Sources []string `json:"sources"` // 来源: "ai500" 和/或 "oi_top"
}

// OITopData 持仓量增长Top数据（用于AI决策参考）
type OITopData struct {
	Rank              int     // OI Top排名
	OIDeltaPercent    float64 // 持仓量变化百分比（1小时）
	OIDeltaValue      float64 // 持仓量变化价值
	PriceDeltaPercent float64 // 价格变化百分比
	NetLong           float64 // 净多仓
	NetShort          float64 // 净空仓
}

// Context 交易上下文（传递给AI的完整信息）
type Context struct {
	CurrentTime     string                  `json:"current_time"`
	RuntimeMinutes  int                     `json:"runtime_minutes"`
	CallCount       int                     `json:"call_count"`
	Account         AccountInfo             `json:"account"`
	Positions       []PositionInfo          `json:"positions"`
	CandidateCoins  []CandidateCoin         `json:"candidate_coins"`
	MarketDataMap   map[string]*market.Data `json:"-"` // 不序列化，但内部使用
	OITopDataMap    map[string]*OITopData   `json:"-"` // OI Top数据映射
	Performance     *PerformanceAnalysis `json:"-"` // 历史表现分析
	BTCETHLeverage  int                     `json:"-"` // BTC/ETH杠杆倍数（从配置读取）
	AltcoinLeverage int                     `json:"-"` // 山寨币杠杆倍数（从配置读取）
	ConfiguredSymbolsToAI []string          `json:"-"` // New field: Symbols configured to be sent to AI
}

// Decision AI的交易决策
type Decision struct {
	Symbol          string  `json:"symbol"`
	Action          string  `json:"action"` // "open_long", "open_short", "close_long", "close_short", "partial_close_long", "partial_close_short", "hold", "wait", "move_sl_to_breakeven"
	Leverage        int     `json:"leverage,omitempty"`
	PositionSizeUSD float64 `json:"position_size_usd,omitempty"`
	StopLoss        float64 `json:"stop_loss,omitempty"`
	TakeProfit      float64 `json:"take_profit,omitempty"`
	NewStopLoss     float64 `json:"new_stop_loss,omitempty"` // For "move_sl_to_breakeven" action
	Confidence      int     `json:"confidence,omitempty"` // 信心度 (0-100)
	RiskUSD         float64 `json:"risk_usd,omitempty"`   // 最大美元风险
	Reasoning       string  `json:"reasoning"`
}

// FullDecision AI的完整决策（包含思维链）
type FullDecision struct {
	UserPrompt string     `json:"user_prompt"` // 发送给AI的输入prompt
	SystemPrompt string   `json:"system_prompt"` // 发送给AI的系统prompt
	CoTTrace   string     `json:"cot_trace"`   // 思维链分析（AI输出）
	Decisions  []Decision `json:"decisions"`   // 具体决策列表
	Timestamp  time.Time  `json:"timestamp"`
}

// GetFullDecision 获取AI的完整交易决策（批量分析所有币种和持仓）
func GetFullDecision(ctx *Context, mcpClient *mcp.Client, customPrompt string, overrideBasePrompt bool, systemPromptTemplate string, promptName string) (*FullDecision, error) {
	// 1. 为所有币种获取市场数据
	if err := fetchMarketDataForContext(ctx); err != nil {
		return nil, fmt.Errorf("获取市场数据失败: %w", err)
	}

	// ADDED LOGGING: Verify ctx.MarketDataMap content
	log.Printf("✅ ctx.MarketDataMap 包含 %d 个币种的市场数据。", len(ctx.MarketDataMap))
	for symbol, data := range ctx.MarketDataMap {
		adx := 0.0
		if data.FourHourContext != nil {
			adx = data.FourHourContext.ADX14
		}
		currentPrice := 0.0
		if data != nil {
			currentPrice = data.CurrentPrice
		}
		log.Printf("   - %s: CurrentPrice=%.2f, 4H_ADX_14=%.2f", symbol, currentPrice, adx)
	}


	// 2. 构建 System Prompt（固定规则）和 User Prompt（动态数据）
	systemPrompt, err := buildSystemPrompt(ctx.Account.TotalEquity, ctx.BTCETHLeverage, ctx.AltcoinLeverage, systemPromptTemplate, customPrompt, overrideBasePrompt, promptName)
	if err != nil {
		return nil, fmt.Errorf("构建系统提示词失败: %w", err)
	}
	userPrompt := buildUserPrompt(ctx)

	var aiResponse string
	var decision *FullDecision
	// var err error // Removed redundant declaration

	const maxRetries = 2 // 首次尝试 + 1次纠错

	for attempt := 0; attempt < maxRetries; attempt++ {
		// 3. 调用AI API
		currentPrompt := userPrompt
		if attempt > 0 {
			// 构建纠错prompt
			correctionPrompt := fmt.Sprintf(
				"Your previous attempt failed with the following error: %v\n\n"+
					"Original Request:\n%s\n\n"+
					"Your Failed Response:\n%s\n\n"+
					"Please review your response, correct the error according to the system rules, and provide the full, corrected response (CoT and JSON).",
				err, // from previous failed attempt
				userPrompt,
				aiResponse,
			)
			currentPrompt = correctionPrompt
			log.Printf("🤖 AI决策验证失败，正在尝试第 %d 次纠错...", attempt)
		}

		aiResponse, err = mcpClient.CallWithMessages(systemPrompt, currentPrompt)
		if err != nil {
			return nil, fmt.Errorf("调用AI API失败 (尝试 %d): %w", attempt+1, err)
		}

		// 4. 解析并验证AI响应
		decision, err = parseFullDecisionResponse(aiResponse, ctx.Account.TotalEquity, ctx.BTCETHLeverage, ctx.AltcoinLeverage)
		if err == nil {
			// 成功，跳出循环
			if attempt > 0 {
				log.Printf("✅ AI决策纠错成功 (尝试 %d)", attempt+1)
			}
			decision.Timestamp = time.Now()
			decision.UserPrompt = userPrompt // 始终保存原始的userPrompt
			decision.SystemPrompt = systemPrompt // Populate the new field
			return decision, nil
		}
	}

	// 如果所有尝试都失败了
	return nil, fmt.Errorf("AI决策在 %d 次尝试后仍然失败: %w", maxRetries, err)
}

// fetchMarketDataForContext 为上下文中的所有币种获取市场数据和OI数据
func fetchMarketDataForContext(ctx *Context) error {
	ctx.MarketDataMap = make(map[string]*market.Data)
	ctx.OITopDataMap = make(map[string]*OITopData)

	// 收集所有需要获取数据的币种
	symbolSet := make(map[string]bool)

	log.Printf("ℹ️ ctx.ConfiguredSymbolsToAI: %v", ctx.ConfiguredSymbolsToAI)

	// Use configured symbols for AI
	for _, symbol := range ctx.ConfiguredSymbolsToAI {
		symbolSet[symbol] = true
	}

	log.Printf("ℹ️ symbolSet after configured symbols: %v", symbolSet)

	// Also ensure existing positions are included if they are in the configured symbols
	for _, pos := range ctx.Positions {
		if _, ok := symbolSet[pos.Symbol]; ok { // Check if the symbol is already in the configured set
			symbolSet[pos.Symbol] = true // Ensure it's marked as true
		}
	}

	log.Printf("ℹ️ symbolSet after existing positions: %v", symbolSet)

	// 持仓币种集合（用于判断是否跳过OI检查）
	positionSymbols := make(map[string]bool)
	for _, pos := range ctx.Positions {
		positionSymbols[pos.Symbol] = true
	}

	// 2. 候选币种数量根据账户状态动态调整 (不再从这里添加，因为我们只关注配置的币种)
	// maxCandidates := calculateMaxCandidates(ctx)
	// for i, coin := range ctx.CandidateCoins {
	// 	if i >= maxCandidates {
	// 		break
	// 	}
	// 	symbolSet[coin.Symbol] = true
	// }

	fetchedCount := 0 // Initialize fetchedCount
	for symbol := range symbolSet {
		data, err := market.Get(symbol)
		if err != nil {
			log.Printf("❌ 获取 %s 市场数据失败: %v", symbol, err)
			continue
		}

		// ⚠️ 流动性过滤：持仓价值低于15M USD的币种不做（多空都不做）
		// 持仓价值 = 持仓量 × 当前价格
		// 但现有持仓必须保留（需要决策是否平仓）
		isExistingPosition := positionSymbols[symbol]
		if !isExistingPosition && data.OpenInterest != nil && data.CurrentPrice > 0 {
			// 计算持仓价值（USD）= 持仓量 × 当前价格
			oiValue := data.OpenInterest.Latest * data.CurrentPrice
			oiValueInMillions := oiValue / 1_000_000 // 转换为百万美元单位
			if oiValueInMillions < 15 {
				log.Printf("⚠️  %s 持仓价值过低(%.2fM USD < 15M)，跳过此币种 [持仓量:%.0f × 价格:%.4f]",
					symbol, oiValueInMillions, data.OpenInterest.Latest, data.CurrentPrice)
				continue
			}
		}

		ctx.MarketDataMap[symbol] = data
		fetchedCount++ // Increment fetchedCount
	}

	if fetchedCount == 0 && len(symbolSet) > 0 {
		return fmt.Errorf("未能获取任何配置币种的市场数据")
	}

	// 加载OI Top数据（不影响主流程）
	oiPositions, err := pool.GetOITopPositions()
	if err == nil {
		for _, pos := range oiPositions {
			// 标准化符号匹配
			symbol := pos.Symbol
			ctx.OITopDataMap[symbol] = &OITopData{
				Rank:              pos.Rank,
				OIDeltaPercent:    pos.OIDeltaPercent,
				OIDeltaValue:      pos.OIDeltaValue,
				PriceDeltaPercent: pos.PriceDeltaPercent,
				NetLong:           pos.NetLong,
				NetShort:          pos.NetShort,
			}
		}
	}

	return nil
}

// calculateMaxCandidates 根据账户状态计算需要分析的候选币种数量
func calculateMaxCandidates(ctx *Context) int {
	// 直接返回候选池的全部币种数量
	// 因为候选池已经在 auto_trader.go 中筛选过了
	// 固定分析前20个评分最高的币种（来自AI500）
	return len(ctx.CandidateCoins)
}

// buildSystemPrompt 构建 System Prompt（固定规则，可缓存）
func buildSystemPrompt(accountEquity float64, btcEthLeverage, altcoinLeverage int, systemPromptTemplate, customPrompt string, overrideBasePrompt bool, promptName string) (string, error) {
	// 如果设置了覆盖基础prompt，则直接使用自定义prompt
	if overrideBasePrompt && customPrompt != "" {
		return customPrompt, nil
	}

	// 获取基础模板
	var basePrompt string
	if promptName == "" {
		promptName = "default" // 默认使用default
	}
	template, err := GetPromptTemplate(promptName)
	if err != nil {
		return "", fmt.Errorf("获取提示词模板 '%s' 失败: %w", promptName, err)
	}
	basePrompt = template.Content

	var sb strings.Builder

	// 写入基础模板
	sb.WriteString(basePrompt)

	return sb.String(), nil
}

// buildUserPrompt 构建 User Prompt（动态数据）
func buildUserPrompt(ctx *Context) string {
	var sb strings.Builder

	// 系统状态
	sb.WriteString(fmt.Sprintf("**时间**: %s | **周期**: #%d | **运行**: %d分钟\n\n",
		ctx.CurrentTime, ctx.CallCount, ctx.RuntimeMinutes))

	// BTC 市场
	if btcData, hasBTC := ctx.MarketDataMap["BTCUSDT"]; hasBTC && btcData.OneHourContext != nil && btcData.IntradaySeries != nil {
		sb.WriteString(fmt.Sprintf("**BTC**: %.2f (1h: %+.2f%%, 4h: %+.2f%%) | MACD: %.4f | RSI: %.2f | 1H_EMA_50: %.4f\n\n",
			btcData.CurrentPrice, btcData.PriceChange1h, btcData.PriceChange4h,
			btcData.IntradaySeries.CurrentMACD, btcData.IntradaySeries.CurrentRSI7, btcData.OneHourContext.EMA50))
	}

	// 账户
	sb.WriteString(fmt.Sprintf("**账户**: 净值%.2f | 余额%.2f (%.1f%%) | 盈亏%+.2f%% | 保证金%.1f%% | 持仓%d个\n\n",
		ctx.Account.TotalEquity,
		ctx.Account.AvailableBalance,
		(ctx.Account.AvailableBalance/ctx.Account.TotalEquity)*100,
		ctx.Account.TotalPnLPct,
		ctx.Account.MarginUsedPct,
		ctx.Account.PositionCount))

	// 持仓（完整市场数据）
	if len(ctx.Positions) > 0 {
		sb.WriteString("## 当前持仓\n")
		for i, pos := range ctx.Positions {
			// 计算持仓时长
			holdingDuration := ""
			if pos.UpdateTime > 0 {
				durationMs := time.Now().UnixMilli() - pos.UpdateTime
				durationMin := durationMs / (1000 * 60) // 转换为分钟
				if durationMin < 60 {
					holdingDuration = fmt.Sprintf(" | 持仓时长%d分钟", durationMin)
				} else {
					durationHour := durationMin / 60
					durationMinRemainder := durationMin % 60
					holdingDuration = fmt.Sprintf(" | 持仓时长%d小时%d分钟", durationHour, durationMinRemainder)
				}
			}

			externalTag := ""
			if pos.IsExternal {
				externalTag = " (外部持仓，请评估)"
			}

			sb.WriteString(fmt.Sprintf("%d. %s %s | 入场价%.4f 当前价%.4f | 盈亏%+.2f%% | 杠杆%dx | 保证金%.0f | 强平价%.4f%s%s\n\n",
				i+1, pos.Symbol, strings.ToUpper(pos.Side),
				pos.EntryPrice, pos.MarkPrice, pos.UnrealizedPnLPct,
				pos.Leverage, pos.MarginUsed, pos.LiquidationPrice, holdingDuration, externalTag))

			// 使用FormatMarketData输出完整市场数据
			if marketData, ok := ctx.MarketDataMap[pos.Symbol]; ok {
				sb.WriteString(market.Format(marketData))
				sb.WriteString("\n")
			}
		}
	} else {
		sb.WriteString("**当前持仓**: 无\n\n")
	}

	// 候选币种（完整市场数据）
	sb.WriteString(fmt.Sprintf("## 候选币种 (%d个)\n\n", len(ctx.MarketDataMap)))
	displayedCount := 0
	for _, coin := range ctx.CandidateCoins {
		marketData, hasData := ctx.MarketDataMap[coin.Symbol]
		if !hasData {
			continue
		}
		displayedCount++

		sourceTags := ""
		if len(coin.Sources) > 1 {
			sourceTags = " (AI500+OI_Top双重信号)"
		} else if len(coin.Sources) == 1 && coin.Sources[0] == "oi_top" {
			sourceTags = " (OI_Top持仓增长)"
		}

		// 使用FormatMarketData输出完整市场数据
		sb.WriteString(fmt.Sprintf("### %d. %s%s\n\n", displayedCount, coin.Symbol, sourceTags))
		sb.WriteString(market.Format(marketData))
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	// 夏普比率（直接传值，不要复杂格式化）
	if ctx.Performance != nil {
		sb.WriteString(fmt.Sprintf("## 📊 夏普比率: %.2f\n\n", ctx.Performance.SharpeRatio))
	}

	sb.WriteString("---\n\n")

	return sb.String()
}

// parseFullDecisionResponse 解析AI的完整决策响应
func parseFullDecisionResponse(aiResponse string, accountEquity float64, btcEthLeverage, altcoinLeverage int) (*FullDecision, error) {
	// 1. 提取思维链
	cotTrace := extractCoTTrace(aiResponse)

	// 2. 提取JSON决策列表
	decisions, err := extractDecisions(aiResponse)
	if err != nil {
		return &FullDecision{
			CoTTrace:  cotTrace,
			Decisions: []Decision{},
		}, fmt.Errorf("提取决策失败: %w\n\n=== AI思维链分析 ===\n%s", err, cotTrace)
	}

	// 3. 验证决策
	if err := validateDecisions(decisions, accountEquity, btcEthLeverage, altcoinLeverage); err != nil {
		return &FullDecision{
			CoTTrace:  cotTrace,
			Decisions: decisions,
		}, fmt.Errorf("决策验证失败: %w\n\n=== AI思维链分析 ===\n%s", err, cotTrace)
	}

	return &FullDecision{
		CoTTrace:  cotTrace,
		Decisions: decisions,
	}, nil
}

// extractCoTTrace 提取思维链分析
func extractCoTTrace(response string) string {
	// 查找JSON代码块的开始标记
	jsonCodeBlockStart := "```json"
	jsonStart := strings.Index(response, jsonCodeBlockStart)

	if jsonStart > 0 {
		// 思维链是JSON代码块开始标记之前的内容
		return strings.TrimSpace(response[:jsonStart])
	}

	// 如果找不到JSON代码块开始标记，整个响应都是思维链
	return strings.TrimSpace(response)
}

// extractDecisions 提取JSON决策列表 (兼容单个对象或数组)
func extractDecisions(response string) ([]Decision, error) {
	// 查找JSON代码块的开始和结束标记
	jsonCodeBlockStart := "```json"
	jsonCodeBlockEnd := "```"

	startIdx := strings.Index(response, jsonCodeBlockStart)
	if startIdx == -1 {
		return nil, fmt.Errorf("无法找到JSON代码块起始标记: %s", jsonCodeBlockStart)
	}

	// 查找结束标记，从起始标记之后开始搜索
	endIdx := strings.Index(response[startIdx+len(jsonCodeBlockStart):], jsonCodeBlockEnd)
	if endIdx == -1 {
		return nil, fmt.Errorf("无法找到JSON代码块结束标记: %s", jsonCodeBlockEnd)
	}
	endIdx += startIdx + len(jsonCodeBlockStart) // 调整endIdx为response中的实际位置

	// 提取JSON内容
	jsonContent := strings.TrimSpace(response[startIdx+len(jsonCodeBlockStart) : endIdx])

	// 🔧 修复常见的JSON格式错误
	jsonContent = fixMissingQuotes(jsonContent)

	// 尝试解析为决策数组
	var decisions []Decision
	err := json.Unmarshal([]byte(jsonContent), &decisions)
	if err == nil {
		return decisions, nil // 成功解析数组
	}

	// 如果数组解析失败，尝试解析为单个决策对象
	var singleDecision Decision
	err2 := json.Unmarshal([]byte(jsonContent), &singleDecision)
	if err2 == nil {
		// 如果单个对象解析成功，将其放入数组中返回
		return []Decision{singleDecision}, nil
	}

	// 如果两种方式都失败，返回原始的数组解析错误
	return nil, fmt.Errorf("JSON解析失败 (尝试数组和对象两种模式后): %w\nJSON内容: %s", err, jsonContent)
}

// fixMissingQuotes 替换中文引号为英文引号（避免输入法自动转换）
func fixMissingQuotes(jsonStr string) string {
	jsonStr = strings.ReplaceAll(jsonStr, "\u201c", "\"") // "
	jsonStr = strings.ReplaceAll(jsonStr, "\u201d", "\"") // "
	jsonStr = strings.ReplaceAll(jsonStr, "\u2018", "'")  // '
	jsonStr = strings.ReplaceAll(jsonStr, "\u2019", "'")  // '
	return jsonStr
}

// validateDecisions 验证所有决策（需要账户信息和杠杆配置）
func validateDecisions(decisions []Decision, accountEquity float64, btcEthLeverage, altcoinLeverage int) error {
	for i, decision := range decisions {
		if err := validateDecision(&decision, accountEquity, btcEthLeverage, altcoinLeverage); err != nil {
			return fmt.Errorf("决策 #%d 验证失败: %w", i+1, err)
		}
	}
	return nil
}

// findMatchingBracket 查找匹配的右括号
func findMatchingBracket(s string, start int) int {
	if start >= len(s) || s[start] != '[' {
		return -1
	}

	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return i
			}
		}
	}

	return -1
}

// validateDecision 验证单个决策的有效性
func validateDecision(d *Decision, accountEquity float64, btcEthLeverage, altcoinLeverage int) error {
	// 验证action
	validActions := map[string]bool{
		"open_long":             true,
		"open_short":            true,
		"close_long":            true,
		"close_short":           true,
		"partial_close_long":    true,
		"partial_close_short":   true,
		"hold":                  true,
		"wait":                  true,
		"move_sl_to_breakeven":  true,
	}

	if !validActions[d.Action] {
		return fmt.Errorf("无效的action: %s", d.Action)
	}

	// 开仓操作必须提供完整参数
	if d.Action == "open_long" || d.Action == "open_short" {
		// 根据币种使用配置的杠杆上限
		maxLeverage := altcoinLeverage          // 山寨币使用配置的杠杆
		maxPositionValue := accountEquity * 1.5 // 山寨币最多1.5倍账户净值
		if d.Symbol == "BTCUSDT" || d.Symbol == "ETHUSDT" {
			maxLeverage = btcEthLeverage          // BTC和ETH使用配置的杠杆
			maxPositionValue = accountEquity * 10 // BTC/ETH最多10倍账户净值
		}

		if d.Leverage <= 0 || d.Leverage > maxLeverage {
			return fmt.Errorf("杠杆必须在1-%d之间（%s，当前配置上限%d倍）: %d", maxLeverage, d.Symbol, maxLeverage, d.Leverage)
		}
		if d.PositionSizeUSD <= 0 {
			return fmt.Errorf("仓位大小必须大于0: %.2f", d.PositionSizeUSD)
		}
		if d.PositionSizeUSD < 20 {
			return fmt.Errorf("仓位价值必须不小于20 USDT: %.2f", d.PositionSizeUSD)
		}
		// 验证仓位价值上限（加1%容差以避免浮点数精度问题）
		tolerance := maxPositionValue * 0.01 // 1%容差
		if d.PositionSizeUSD > maxPositionValue+tolerance {
			if d.Symbol == "BTCUSDT" || d.Symbol == "ETHUSDT" {
				return fmt.Errorf("BTC/ETH单币种仓位价值不能超过%.0f USDT（10倍账户净值），实际: %.0f", maxPositionValue, d.PositionSizeUSD)
			} else {
				return fmt.Errorf("山寨币单币种仓位价值不能超过%.0f USDT（1.5倍账户净值），实际: %.0f", maxPositionValue, d.PositionSizeUSD)
			}
		}
		if d.StopLoss <= 0 || d.TakeProfit <= 0 {
			return fmt.Errorf("止损和止盈必须大于0")
		}

		// 验证止损止盈的合理性
		if d.Action == "open_long" {
			if d.StopLoss >= d.TakeProfit {
				return fmt.Errorf("做多时止损价必须小于止盈价")
			}
		} else {
			if d.StopLoss <= d.TakeProfit {
				return fmt.Errorf("做空时止损价必须大于止盈价")
			}
		}

		var riskPercent, rewardPercent, riskRewardRatio float64
		marketData, err := market.Get(d.Symbol)
		if err != nil {
			return fmt.Errorf("获取当前市场数据失败: %w", err)
		}
		currentMarketPrice := marketData.CurrentPrice

		// 验证止损止盈的合理性
		if d.Action == "open_long" {
			if d.StopLoss >= currentMarketPrice {
				return fmt.Errorf("做多时止损价(%.4f)必须小于当前市场价(%.4f)", d.StopLoss, currentMarketPrice)
			}
			if d.TakeProfit <= currentMarketPrice {
				return fmt.Errorf("做多时止盈价(%.4f)必须大于当前市场价(%.4f)", d.TakeProfit, currentMarketPrice)
			}
		} else { // open_short
			if d.StopLoss <= currentMarketPrice {
				return fmt.Errorf("做空时止损价(%.4f)必须大于当前市场价(%.4f)", d.StopLoss, currentMarketPrice)
			}
			if d.TakeProfit >= currentMarketPrice {
				return fmt.Errorf("做空时止盈价(%.4f)必须小于当前市场价(%.4f)", d.TakeProfit, currentMarketPrice)
			}
		}

		if d.Action == "open_long" {
			riskPercent = (currentMarketPrice - d.StopLoss) / currentMarketPrice * 100
			rewardPercent = (d.TakeProfit - currentMarketPrice) / currentMarketPrice * 100
			if riskPercent > 0 {
				riskRewardRatio = rewardPercent / riskPercent
			}
		} else {
			riskPercent = (d.StopLoss - currentMarketPrice) / currentMarketPrice * 100
			rewardPercent = (currentMarketPrice - d.TakeProfit) / currentMarketPrice * 100
			if riskPercent > 0 {
				riskRewardRatio = rewardPercent / riskPercent
			}
		}

		// 硬约束：风险回报比必须≥1.9
		if riskRewardRatio < 1.9 {
			return fmt.Errorf("风险回报比过低(%.2f:1)，必须≥1.9:1 [风险:%.2f%% 收益:%.2f%%] [止损:%.2f 止盈:%.2f]",
				riskRewardRatio, riskPercent, rewardPercent, d.StopLoss, d.TakeProfit)
		}
	} else if d.Action == "move_sl_to_breakeven" {
		if d.NewStopLoss <= 0 {
			return fmt.Errorf("移动止损价(NewStopLoss)必须大于0")
		}
	}

	return nil
}
