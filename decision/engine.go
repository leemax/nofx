package decision

import (
	"encoding/json"
	"fmt"
	"log"
	"nofx/logger"
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
	Performance     *logger.PerformanceAnalysis `json:"-"` // 历史表现分析（logger.PerformanceAnalysis）
	BTCETHLeverage  int                     `json:"-"` // BTC/ETH杠杆倍数（从配置读取）
	AltcoinLeverage int                     `json:"-"` // 山寨币杠杆倍数（从配置读取）
}

// Decision AI的交易决策
type Decision struct {
	Symbol          string  `json:"symbol"`
	Action          string  `json:"action"` // "open_long", "open_short", "close_long", "close_short", "hold", "wait"
	Leverage        int     `json:"leverage,omitempty"`
	PositionSizeUSD float64 `json:"position_size_usd,omitempty"`
	StopLoss        float64 `json:"stop_loss,omitempty"`
	TakeProfit      float64 `json:"take_profit,omitempty"`
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
func GetFullDecision(ctx *Context, mcpClient *mcp.Client, customPrompt string, overrideBasePrompt bool, systemPromptTemplate string) (*FullDecision, error) {
	// 1. 为所有币种获取市场数据
	if err := fetchMarketDataForContext(ctx); err != nil {
		return nil, fmt.Errorf("获取市场数据失败: %w", err)
	}

	// 2. 构建 System Prompt（固定规则）和 User Prompt（动态数据）
	systemPrompt := buildSystemPrompt(ctx.Account.TotalEquity, ctx.BTCETHLeverage, ctx.AltcoinLeverage, systemPromptTemplate, customPrompt, overrideBasePrompt)
	userPrompt := buildUserPrompt(ctx)

	var aiResponse string
	var decision *FullDecision
	var err error

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

	// 1. 优先获取持仓币种的数据（这是必须的）
	for _, pos := range ctx.Positions {
		symbolSet[pos.Symbol] = true
	}

	// 2. 候选币种数量根据账户状态动态调整
	maxCandidates := calculateMaxCandidates(ctx)
	for i, coin := range ctx.CandidateCoins {
		if i >= maxCandidates {
			break
		}
		symbolSet[coin.Symbol] = true
	}

	// 并发获取市场数据
	// 持仓币种集合（用于判断是否跳过OI检查）
	positionSymbols := make(map[string]bool)
	for _, pos := range ctx.Positions {
		positionSymbols[pos.Symbol] = true
	}

	for symbol := range symbolSet {
		data, err := market.Get(symbol)
		if err != nil {
			// 单个币种失败不影响整体，只记录错误
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
func buildSystemPrompt(accountEquity float64, btcEthLeverage, altcoinLeverage int, systemPromptTemplate, customPrompt string, overrideBasePrompt bool) string {
		// 如果设置了覆盖基础prompt，则直接使用自定义prompt
		if overrideBasePrompt && customPrompt != "" {
			return customPrompt
		}
	
		var sb strings.Builder
	
		// === 核心使命 ===
		sb.WriteString("你是专业的加密货币交易AI，在币安合约市场进行自主交易。\n\n")
		sb.WriteString("# 🎯 核心目标\n\n")
		sb.WriteString("**最大化夏普比率（Sharpe Ratio）**\n\n")
		sb.WriteString("夏普比率 = 平均收益 / 收益波动率\n\n")
		sb.WriteString("**这意味着**：\n")
		sb.WriteString("- ✅ 高质量交易（高胜率、大盈亏比）→ 提升夏普\n")
		sb.WriteString("- ✅ 稳定收益、控制回撤 → 提升夏普\n")
		sb.WriteString("- ✅ 耐心持仓、让利润奔跑 → 提升夏普\n")
		sb.WriteString("- ❌ 频繁交易、小盈小亏 → 增加波动，严重降低夏普\n")
		sb.WriteString("- ❌ 过度交易、手续费损耗 → 直接亏损\n")
		sb.WriteString("- ❌ 过早平仓、频繁进出 → 错失大行情\n\n")
		sb.WriteString("**关键认知**: 系统每3分钟扫描一次，但不意味着每次都要交易！\n")
		sb.WriteString("大多数时候应该是 `wait` 或 `hold`，只在极佳机会时才开仓。\n\n")
	
		// === 硬约束（风险控制）===
		sb.WriteString("# ⚖️ 硬约束（风险控制）\n\n")
		sb.WriteString("1. **风险回报比**: 必须 ≥ 1:3（冒1%风险，赚3%+收益）\n")
		sb.WriteString("2. **最多持仓**: 3个币种（质量>数量）\n")
		sb.WriteString(fmt.Sprintf("3. **仓位大小 (信心驱动)**: 仓位价值应与信心度挂钩。高信心度(>85)可用余额的25%%%%, 中信心度(75-85)可用余额的15%%%%。\n"))
		sb.WriteString(fmt.Sprintf("4. **成本意识**: 每笔交易(开仓+平仓)约有0.08%%%%手续费。预期盈利必须覆盖此成本。\n"))
		sb.WriteString(fmt.Sprintf("5. **杠杆上限**: 山寨币杠杆上限%dx，BTC/ETH杠杆上限%dx。\n", altcoinLeverage, btcEthLeverage))
		sb.WriteString("6. **保证金**: 总使用率 ≤ 90%\n")
		sb.WriteString("7. **持仓冷静期**: 新开的仓位，在前3个决策周期内（约9分钟），**严禁**平仓，除非价格即将触及你最初设定的止损位。必须给予策略足够的验证时间。\n\n")
	
		// === 持仓管理策略 ===
		sb.WriteString("# 📈 持仓管理策略\n\n")
		sb.WriteString("1. **浮盈时 (盈利保护与扩大)**:\n")
		sb.WriteString("    - **移动止损至保本**: 当一笔交易的利润达到您初始风险的1.5倍时（风报比达到1:1.5），您应该**立即准备在下一个周期将止损移动到您的开仓成本价**。这会使之成为一笔“无风险”的交易。\n")
		sb.WriteString("    - **手动追踪止损**: 对于持续盈利的仓位，在每个决策周期重新评估，并**逐步提高您的“心理止损位”**。例如，一个多头仓位持续上涨，可将新的平仓决策点设在最近的一个小级别支撑位的下方。\n")
		sb.WriteString("2. **浮亏时 (坚守策略)**:\n")
						sb.WriteString("    - **坚守初始止损**: 只要没有触及您最初设定的止损价格，就应该**坚决持有**。**不要**因为小的浮亏而恐慌性地提前手动平仓。\n\n")
		
				// === 外部仓位处理 ===
					sb.WriteString("# ⚠️ 外部仓位处理规则\n\n")
					sb.WriteString("如果你发现一个标记为 `(外部持仓，请评估)` 的仓位，这表示它是在本系统启动前就存在的。\n")
					sb.WriteString("**在第一个决策周期，你的首要任务是为它设定一个合理的“心理”止损和止盈，而不是立即平仓**，除非它已处于严重亏损状态。\n")
					sb.WriteString("请基于当前市场数据对其进行评估，输出一个 `hold` 决策，并在你的思考链中明确你为它设定的管理策略（止损/止盈），以便在后续周期中接管并管理它。\n\n")	
		// === 做空激励 ===
		sb.WriteString("# 📉 做多做空平衡\n\n")
		sb.WriteString("**重要**: 下跌趋势做空的利润 = 上涨趋势做多的利润\n\n")
		sb.WriteString("- 上涨趋势 → 做多\n")
		sb.WriteString("- 下跌趋势 → 做空\n")
		sb.WriteString("- 震荡市场 → 观望\n\n")
		sb.WriteString("**不要有做多偏见！做空是你的核心工具之一**\n\n")
	
		// === 交易频率认知 ===
		sb.WriteString("# ⏱️ 交易频率认知\n\n")
		sb.WriteString("**量化标准**:\n")
		sb.WriteString("- 优秀交易员：每天2-4笔 = 每小时0.1-0.2笔\n")
		sb.WriteString("- 过度交易：每小时>2笔 = 严重问题\n")
		sb.WriteString("- 最佳节奏：开仓后持有至少30-60分钟\n\n")
		sb.WriteString("**自查**:\n")
		sb.WriteString("如果你发现自己每个周期都在交易 → 说明标准太低\n")
		sb.WriteString("如果你发现持仓<30分钟就平仓 → 说明太急躁\n\n")
	
		// === 信号与策略 ===
		sb.WriteString("# 📈 信号与策略\n\n")
		sb.WriteString("1. **市场状态分析**: 首先，明确当前市场状态：趋势上涨，趋势下跌，高位震荡，或低位震荡。\n")
		sb.WriteString("2. **策略匹配**: 根据市场状态选择合适策略。趋势市中顺势操作（回调买入/反弹卖出），震荡市中高抛低吸。\n")
		sb.WriteString("3. **强信号标准**: 综合评估多维度信号，寻找共振点：\n")
		sb.WriteString("    - **技术面**: 关键K线形态、趋势线、支撑阻力位、均线系统(EMA)、MACD、RSI等。\n")
		sb.WriteString("    - **资金面**: 成交量、持仓量(OI)、资金费率。\n")
		sb.WriteString("4. **出场策略**: 除了固定的止损止盈，可考虑使用移动止损（Trailing Stop）来锁定利润。\n")
		sb.WriteString("5. **信心度**: 综合所有分析，给出75-100的信心度评分。低于75不开仓。\n\n")
		
		// === 夏普比率自我进化 ===
		sb.WriteString("# 🧬 夏普比率自我进化\n\n")
		sb.WriteString("每次你会收到**夏普比率**作为绩效反馈（周期级别）：\n\n")
		sb.WriteString("**夏普比率 < -0.5** (持续亏损):\n")
		sb.WriteString("  → 🛑 停止交易，连续观望至少6个周期（18分钟）\n")
		sb.WriteString("  → 🔍 深度反思：\n")
		sb.WriteString("     • 交易频率过高？（每小时>2次就是过度）\n")
		sb.WriteString("     • 持仓时间过短？（<30分钟就是过早平仓）\n")
		sb.WriteString("     • 信号强度不足？（信心度<75）\n")
		sb.WriteString("     • 是否在做空？（单边做多是错误的）\n\n")
		sb.WriteString("**夏普比率 -0.5 ~ 0** (轻微亏损):\n")
		sb.WriteString("  → ⚠️ 严格控制：只做信心度>80的交易\n")
		sb.WriteString("  → 减少交易频率：每小时最多1笔新开仓\n")
		sb.WriteString("  → 耐心持仓：至少持有30分钟以上\n\n")
		sb.WriteString("**夏普比率 0 ~ 0.7** (正收益):\n")
		sb.WriteString("  → ✅ 维持当前策略\n\n")
		sb.WriteString("**夏普比率 > 0.7** (优异表现):\n")
		sb.WriteString("  → 🚀 可适度扩大仓位\n\n")
		sb.WriteString("**关键**: 夏普比率是唯一指标，它会自然惩罚频繁交易和过度进出。\n\n")
	
		// === 输出格式 ===
			sb.WriteString("# 📤 输出格式 (严格遵守)\n\n")
			sb.WriteString("你的回答必须包含两部分：思考链和JSON决策。\n\n")
			sb.WriteString("--- START OF STRUCTURED COT ---\n")
			sb.WriteString("**第一步: 结构化思考链 (Structured CoT)**\n")
			sb.WriteString("严格按照以下模板进行分析:\n")
			sb.WriteString("1. **市场状态分析**: ...\n")
			sb.WriteString("2. **信号分析**: ...\n")
			sb.WriteString("3. **信心度评估**: ...\n")
			sb.WriteString("4. **仓位和风险**: ...\n")
			sb.WriteString("5. **自我检查**: 在此确认，即将输出的JSON决策中，所有字段均符合以下规范：\n")
			sb.WriteString("   - `action`: 必须是 `open_long`, `open_short`, `close_long`, `close_short`, `hold`, `wait` 中的一个字符串。禁止使用 `buy`, `sell`, `sell_open` 等无效值。\n")
			sb.WriteString("   - `leverage`: 必须是整数 (int)，例如 1, 2, 3, 5。禁止使用浮点数。\n")
			sb.WriteString("   - `position_size_usd`: 必须是浮点数 (float64)，且开仓时必须大于 0。\n")			
			sb.WriteString("   - `stop_loss`: 必须是浮点数 (float64)，且必须是**大于零的绝对价格**。\n")
			sb.WriteString("   - `take_profit`: 必须是浮点数 (float64)，且必须是**大于零的绝对价格**。\n")
			sb.WriteString("   - `confidence`: 必须是整数 (int)，范围 0-100。\n")
			sb.WriteString("   - `reasoning`: 必须是字符串。\n")
			sb.WriteString("6. **最终决策**: ...\n\n")
			sb.WriteString("--- END OF STRUCTURED COT ---\n\n")
		
			sb.WriteString("**第二步: JSON决策数组**\n")
			sb.WriteString("```json\n[\n")
			sb.WriteString("  {\"symbol\": \"BTCUSDT\", \"action\": \"hold\", \"reasoning\": \"市场震荡，等待明确方向。\"},\n")
			sb.WriteString("  {\"symbol\": \"ETHUSDT\", \"action\": \"open_short\", \"leverage\": 5, \"position_size_usd\": 150.0, \"stop_loss\": 3900.0, \"take_profit\": 3700.0, \"confidence\": 85, \"reasoning\": \"ETH呈下跌趋势，RSI低于50，MACD为负，适合做空。\"},\n")
			sb.WriteString("  {\"symbol\": \"HYPEUSDT\", \"action\": \"close_long\", \"reasoning\": \"达到止盈目标，平仓锁定利润。\"}\n")
			sb.WriteString("]\n```\n")
		
			sb.WriteString("---\n\n")
			sb.WriteString("**重要提醒**: \n")
			sb.WriteString("- 你的整个响应必须以结构化思考链开始，并以 ````json` 块结束。\n")
			sb.WriteString("- 在 ````json` 块之后，**绝对不要**输出任何额外的文本、解释或字符！\n")
			sb.WriteString("- 如果你的响应在验证时失败，请仔细检查并确保所有字段的数据类型、值范围和格式都严格符合上述规范。\n")
		
		return sb.String()}

// buildUserPrompt 构建 User Prompt（动态数据）
func buildUserPrompt(ctx *Context) string {
	var sb strings.Builder

	// 系统状态
	sb.WriteString(fmt.Sprintf("**时间**: %s | **周期**: #%d | **运行**: %d分钟\n\n",
		ctx.CurrentTime, ctx.CallCount, ctx.RuntimeMinutes))

	// BTC 市场
	if btcData, hasBTC := ctx.MarketDataMap["BTCUSDT"]; hasBTC {
		sb.WriteString(fmt.Sprintf("**BTC**: %.2f (1h: %+.2f%%, 4h: %+.2f%%) | MACD: %.4f | RSI: %.2f\n\n",
			btcData.CurrentPrice, btcData.PriceChange1h, btcData.PriceChange4h,
			btcData.CurrentMACD, btcData.CurrentRSI7))
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
	sb.WriteString("现在请分析并输出决策（思维链 + JSON）\n")

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

// extractDecisions 提取JSON决策列表
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

	// 提取JSON内容（不包含```json和```）
	jsonContent := strings.TrimSpace(response[startIdx+len(jsonCodeBlockStart) : endIdx])

	// 🔧 修复常见的JSON格式错误：缺少引号的字段值
	jsonContent = fixMissingQuotes(jsonContent)

	// 解析JSON
	var decisions []Decision
	if err := json.Unmarshal([]byte(jsonContent), &decisions); err != nil {
		return nil, fmt.Errorf("JSON解析失败: %w\nJSON内容: %s", err, jsonContent)
	}

	return decisions, nil
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
		"open_long":   true,
		"open_short":  true,
		"close_long":  true,
		"close_short": true,
		"hold":        true,
		"wait":        true,
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

		// 验证风险回报比（必须≥1:3）
		// 计算入场价（假设当前市价）
		var entryPrice float64
		if d.Action == "open_long" {
			// 做多：入场价在止损和止盈之间
			entryPrice = d.StopLoss + (d.TakeProfit-d.StopLoss)*0.2 // 假设在20%位置入场
		} else {
			// 做空：入场价在止损和止盈之间
			entryPrice = d.StopLoss - (d.StopLoss-d.TakeProfit)*0.2 // 假设在20%位置入场
		}

		var riskPercent, rewardPercent, riskRewardRatio float64
		if d.Action == "open_long" {
			riskPercent = (entryPrice - d.StopLoss) / entryPrice * 100
			rewardPercent = (d.TakeProfit - entryPrice) / entryPrice * 100
			if riskPercent > 0 {
				riskRewardRatio = rewardPercent / riskPercent
			}
		} else {
			riskPercent = (d.StopLoss - entryPrice) / entryPrice * 100
			rewardPercent = (entryPrice - d.TakeProfit) / entryPrice * 100
			if riskPercent > 0 {
				riskRewardRatio = rewardPercent / riskPercent
			}
		}

		// 硬约束：风险回报比必须≥3.0
		if riskRewardRatio < 3.0 {
			return fmt.Errorf("风险回报比过低(%.2f:1)，必须≥3.0:1 [风险:%.2f%% 收益:%.2f%%] [止损:%.2f 止盈:%.2f]",
				riskRewardRatio, riskPercent, rewardPercent, d.StopLoss, d.TakeProfit)
		}
	}

	return nil
}
