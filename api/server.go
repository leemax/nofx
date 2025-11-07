package api

import (
	"fmt"
	"log"
	"net/http"
	"nofx/config"
	"nofx/database"
	"nofx/decision"
	"nofx/manager"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// PromptResponse 用于API返回给前端的提示词结构
type PromptResponse struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Text    string `json:"text"`
}

// Server HTTP API服务器
type Server struct {
	router        *gin.Engine
	traderManager *manager.TraderManager
	config        *config.Config // 新增配置引用
	port          int
}

// NewServer 创建API服务器
func NewServer(traderManager *manager.TraderManager, cfg *config.Config, port int) *Server {
	// 设置为Release模式（减少日志输出）
	gin.SetMode(gin.ReleaseMode)

	router := gin.Default()

	// 配置并启用CORS中间件
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowOrigins = []string{"http://localhost:3000", "http://43.153.176.90", "http://43.153.176.90:3000"} // 明确允许前端的源
	corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	corsConfig.AllowHeaders = []string{"Origin", "Content-Type", "Authorization"}
	router.Use(cors.New(corsConfig))

	s := &Server{
		router:        router,
		traderManager: traderManager,
		config:        cfg,
		port:          port,
	}

	// 设置路由
	s.setupRoutes()

	return s
}

// setupRoutes 设置路由
func (s *Server) setupRoutes() {
	// 健康检查
	s.router.Any("/health", s.handleHealth)

	// API路由组
	api := s.router.Group("/api")
	{
		// 竞赛总览
		api.GET("/competition", s.handleCompetition)

		// Trader列表
		api.GET("/traders", s.handleTraderList)

				api.GET("/statistics", s.handleStatistics)
		api.GET("/status", s.handleStatus)
		api.GET("/account", s.handleAccount)
		api.GET("/positions", s.handlePositions)
		api.GET("/decisions", s.handleDecisions)
		api.GET("/decisions/latest", s.handleLatestDecisions)

		api.GET("/equity-history", s.handleEquityHistory)
		api.GET("/performance", s.handlePerformance)

		// 交易记录
		api.GET("/trades", s.handleTrades)
		// 新增：已平仓交易的盈亏分析
		api.GET("/closed-positions", s.handleClosedPositions)

		    // 新增：获取系统提示词列表
		    api.GET("/prompts", s.handleGetPrompts)
		
		    // 新增：手动触发决策
		    api.POST("/force-decision", s.handleForceDecision)

		    // 新增：设置trader的默认提示词
		    api.POST("/trader/prompt", s.handleSetTraderPrompt)

		    // 新增：启动/停止trader
		    api.POST("/trader/:traderId/start", s.handleStartTrader)
		    api.POST("/trader/:traderId/stop", s.handleStopTrader)
		    api.POST("/trader/:traderId/set-decision-maker", s.handleSetDecisionMaker)
		  }
		}
// handleClosedPositions 处理已平仓交易的请求
func (s *Server) handleClosedPositions(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 从数据库获取所有交易记录，按时间升序
	trades, err := database.GetTrades(traderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取交易记录失败: %v", err),
		})
		return
	}

	// 调用头寸匹配算法
	closedPositions := decision.MatchPositions(trades)

	c.JSON(http.StatusOK, closedPositions)
}

// handleGetPrompts 获取所有系统提示词
func (s *Server) handleGetPrompts(c *gin.Context) {
	templates := decision.GetAllPromptTemplates()
	
	var promptResponses []PromptResponse
	for _, tpl := range templates {
		promptResponses = append(promptResponses, PromptResponse{
			ID:      tpl.ID,
			Name:    tpl.Name,
			Text:    tpl.Content,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"prompts": promptResponses,
		"default_prompt_id": s.config.DefaultPrompt,
	})
}

// handleForceDecision 手动触发一次决策
func (s *Server) handleForceDecision(c *gin.Context) {
	traderID := c.Query("trader_id")
	if traderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "trader_id is required"})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	if err := trader.ForceDecision(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("强制决策失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "决策已触发"})
}

// handleTrades 交易记录
func (s *Server) handleTrades(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trades, err := database.GetTrades(traderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取交易记录失败: %v", err),
		})
		return
	}

		// 聚合交易记录
	aggregatedTrades := make(map[int64]database.TradeRecord)
	for _, trade := range trades {
		if existingTrade, ok := aggregatedTrades[trade.OrderID]; ok {
			// 如果已经存在该订单的记录，则聚合
			// 计算加权平均价
			newQuantity := existingTrade.Quantity + trade.Quantity
			if newQuantity > 0 {
				existingTrade.Price = (existingTrade.Price*existingTrade.Quantity + trade.Price*trade.Quantity) / newQuantity
			}
			existingTrade.Quantity = newQuantity
			existingTrade.Commission += trade.Commission
			// 时间戳更新为最近的成交时间
			if trade.Timestamp.After(existingTrade.Timestamp) {
				existingTrade.Timestamp = trade.Timestamp
			}
			aggregatedTrades[trade.OrderID] = existingTrade
		} else {
			// 否则，直接添加
			aggregatedTrades[trade.OrderID] = trade
		}
	}

	// 将聚合后的map转换为slice
	var result []database.TradeRecord
	for _, trade := range aggregatedTrades {
		result = append(result, trade)
	}

	c.JSON(http.StatusOK, result)
}

// handleHealth 健康检查
func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"time":   c.Request.Context().Value("time"),
	})
}

// getTraderFromQuery 从query参数获取trader
func (s *Server) getTraderFromQuery(c *gin.Context) (*manager.TraderManager, string, error) {
	traderID := c.Query("trader_id")
	if traderID == "" {
		// 如果没有指定trader_id，返回第一个trader
		ids := s.traderManager.GetTraderIDs()
		if len(ids) == 0 {
			return nil, "", fmt.Errorf("没有可用的trader")
		}
		traderID = ids[0]
	}
	return s.traderManager, traderID, nil
}

// handleCompetition 竞赛总览（对比所有trader）
func (s *Server) handleCompetition(c *gin.Context) {
	comparison, err := s.traderManager.GetComparisonData()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取对比数据失败: %v", err),
		})
		return
	}
	c.JSON(http.StatusOK, comparison)
}

// handleTraderList trader列表
func (s *Server) handleTraderList(c *gin.Context) {
	traders := s.traderManager.GetAllTraders()
	result := make([]map[string]interface{}, 0, len(traders))

	for _, t := range traders {
		result = append(result, map[string]interface{}{
			"trader_id":   t.GetID(),
			"trader_name": t.GetName(),
			"ai_model":    t.GetAIModel(),
		})
	}

	c.JSON(http.StatusOK, result)
}

// handleStatus 系统状态
func (s *Server) handleStatus(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	status := trader.GetStatus()
	c.JSON(http.StatusOK, status)
}

// handleStatistics 获取统计信息
func (s *Server) handleStatistics(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	statistics := trader.GetStatistics()
	c.JSON(http.StatusOK, statistics)
}

// handleAccount 账户信息
func (s *Server) handleAccount(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	log.Printf("📊 收到账户信息请求 [%s]", trader.GetName())
	account, err := trader.GetAccountInfo()
	if err != nil {
		log.Printf("❌ 获取账户信息失败 [%s]: %v", trader.GetName(), err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取账户信息失败: %v", err),
		})
		return
	}

	log.Printf("✓ 返回账户信息 [%s]: 净值=%.2f, 可用=%.2f, 盈亏=%.2f (%.2f%%)",
		trader.GetName(),
		account["total_equity"],
		account["available_balance"],
		account["total_pnl"],
		account["total_pnl_pct"])
	c.JSON(http.StatusOK, account)
}

// handlePositions 持仓列表
func (s *Server) handlePositions(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	positions, err := trader.GetPositions()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取持仓列表失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, positions)
}

// handleDecisions 决策日志列表
func (s *Server) handleDecisions(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// 获取所有历史决策记录（无限制）
	records, err := trader.GetDecisionLogger().GetLatestRecords(10000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取决策日志失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, records)
}

// handleLatestDecisions 最新决策日志（最近5条，最新的在前）
func (s *Server) handleLatestDecisions(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	records, err := trader.GetDecisionLogger().GetLatestRecords(20)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取决策日志失败: %v", err),
		})
		return
	}

	// 反转数组，让最新的在前面（用于列表显示）
	// GetLatestRecords返回的是从旧到新（用于图表），这里需要从新到旧
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}

	c.JSON(http.StatusOK, records)
}


// handleEquityHistory 收益率历史数据
func (s *Server) handleEquityHistory(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// 获取尽可能多的历史数据（几天的数据）
	// 每3分钟一个周期：10000条 = 约20天的数据
	records, err := trader.GetDecisionLogger().GetLatestRecords(10000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取历史数据失败: %v", err),
		})
		return
	}

	// 构建收益率历史数据点
	type EquityPoint struct {
		Timestamp        string  `json:"timestamp"`
		TotalEquity      float64 `json:"total_equity"`      // 账户净值（wallet + unrealized）
		AvailableBalance float64 `json:"available_balance"` // 可用余额
		TotalPnL         float64 `json:"total_pnl"`         // 总盈亏（相对初始余额）
		TotalPnLPct      float64 `json:"total_pnl_pct"`     // 总盈亏百分比
		PositionCount    int     `json:"position_count"`    // 持仓数量
		MarginUsedPct    float64 `json:"margin_used_pct"`   // 保证金使用率
		CycleNumber      int     `json:"cycle_number"`
	}

	// 从AutoTrader获取初始余额（用于计算盈亏百分比）
	initialBalance := 0.0
	if status := trader.GetStatus(); status != nil {
		if ib, ok := status["initial_balance"].(float64); ok && ib > 0 {
			initialBalance = ib
		}
	}

	// 如果无法从status获取，且有历史记录，则从第一条记录获取
	if initialBalance == 0 && len(records) > 0 {
		// 第一条记录的equity作为初始余额
		initialBalance = records[0].AccountState.TotalBalance
	}

	// 如果还是无法获取，返回错误
	if initialBalance == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "无法获取初始余额",
		})
		return
	}

	var history []EquityPoint
	for _, record := range records {
		// TotalBalance字段实际存储的是TotalEquity
		totalEquity := record.AccountState.TotalBalance
		// TotalUnrealizedProfit字段实际存储的是TotalPnL（相对初始余额）
		totalPnL := record.AccountState.TotalUnrealizedProfit

		// 计算盈亏百分比
		totalPnLPct := 0.0
		if initialBalance > 0 {
			totalPnLPct = (totalPnL / initialBalance) * 100
		}

		history = append(history, EquityPoint{
			Timestamp:        record.Timestamp.Format("2006-01-02 15:04:05"),
			TotalEquity:      totalEquity,
			AvailableBalance: record.AccountState.AvailableBalance,
			TotalPnL:         totalPnL,
			TotalPnLPct:      totalPnLPct,
			PositionCount:    record.AccountState.PositionCount,
			MarginUsedPct:    record.AccountState.MarginUsedPct,
			CycleNumber:      record.CycleNumber,
		})
	}

	c.JSON(http.StatusOK, history)
}

// handlePerformance AI历史表现分析（用于展示AI学习和反思）
func (s *Server) handlePerformance(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	records, err := trader.GetDecisionLogger().GetLatestRecords(10000) // Get all records for performance analysis
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取决策日志失败: %v", err),
		})
		return
	}

	performance, err := decision.Analyze(traderID, records)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("分析历史表现失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, performance)
}

// Start 启动服务器
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("🌐 API服务器启动在 http://localhost%s", addr)
	log.Printf("📊 API文档:")
	log.Printf("  • GET  /api/competition      - 竞赛总览（对比所有trader）")
	log.Printf("  • GET  /api/traders          - Trader列表")
	log.Printf("  • GET  /api/status?trader_id=xxx     - 指定trader的系统状态")
	log.Printf("  • GET  /api/account?trader_id=xxx    - 指定trader的账户信息")
	log.Printf("  • GET  /api/positions?trader_id=xxx  - 指定trader的持仓列表")
	log.Printf("  • GET  /api/decisions?trader_id=xxx  - 指定trader的决策日志")
	log.Printf("  • GET  /api/decisions/latest?trader_id=xxx - 指定trader的最新决策")
	log.Printf("  • GET  /api/statistics?trader_id=xxx - 指定trader的统计信息")
	log.Printf("  • GET  /api/equity-history?trader_id=xxx - 指定trader的收益率历史数据")
	log.Printf("  • GET  /api/performance?trader_id=xxx - 指定trader的AI学习表现分析")
	log.Printf("  • GET  /health               - 健康检查")
	log.Println()

	return s.router.Run(addr)
}

// handleSetTraderPrompt sets the default prompt for a specific trader
func (s *Server) handleSetTraderPrompt(c *gin.Context) {
	var req struct {
		TraderID   string `json:"trader_id"`
		PromptName string `json:"prompt_name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if err := s.traderManager.SetTraderPrompt(req.TraderID, req.PromptName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "Prompt updated successfully"})
}

// handleStartTrader 启动trader
func (s *Server) handleStartTrader(c *gin.Context) {
	traderID := c.Param("traderId")
	if traderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "traderId is required"})
		return
	}

	if err := s.traderManager.StartTrader(traderID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("启动交易员失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "交易员已启动"})
}

// handleStopTrader 停止trader
func (s *Server) handleStopTrader(c *gin.Context) {
	traderID := c.Param("traderId")
	if traderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "traderId is required"})
		return
	}

	if err := s.traderManager.StopTrader(traderID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("停止交易员失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "交易员已停止"})
}

// handleSetDecisionMaker 设置唯一的决策者
func (s *Server) handleSetDecisionMaker(c *gin.Context) {
	traderID := c.Param("traderId")
	if traderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "traderId is required"})
		return
	}

	if err := s.traderManager.SetDecisionMaker(traderID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("设置决策者失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "决策者已设置"})
}
