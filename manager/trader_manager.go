package manager

import (
	"fmt"
	"log"
	"nofx/config"
	"nofx/trader"
	"sync"
	"time"
)

// TraderManager 管理多个trader实例
type TraderManager struct {
	traders map[string]*trader.AutoTrader // key: trader ID
	config  *config.Config                // Store the global config
	mu      sync.RWMutex
}

// NewTraderManager 创建trader管理器
func NewTraderManager(cfg *config.Config) *TraderManager {
	return &TraderManager{
		traders: make(map[string]*trader.AutoTrader),
		config:  cfg,
	}
}

// AddTrader 添加一个trader
func (tm *TraderManager) AddTrader(traderID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	var cfg config.TraderConfig
	found := false
	for _, tCfg := range tm.config.Traders {
		if tCfg.ID == traderID {
			cfg = tCfg
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("trader ID '%s' not found in config", traderID)
	}

	if _, exists := tm.traders[cfg.ID]; exists {
		return fmt.Errorf("trader ID '%s' 已存在", cfg.ID)
	}

	// 构建AutoTraderConfig
	traderConfig := trader.AutoTraderConfig{
		ID:                    cfg.ID,
		Name:                  cfg.Name,
		AIModel:               cfg.AIModel,
		Exchange:              cfg.Exchange,
		BinanceAPIKey:         cfg.BinanceAPIKey,
		BinanceSecretKey:      cfg.BinanceSecretKey,
		HyperliquidPrivateKey: cfg.HyperliquidPrivateKey,
		HyperliquidWalletAddr: cfg.HyperliquidWalletAddr,
		HyperliquidTestnet:    cfg.HyperliquidTestnet,
		AsterUser:             cfg.AsterUser,
		AsterSigner:           cfg.AsterSigner,
		AsterPrivateKey:       cfg.AsterPrivateKey,
		CoinPoolAPIURL:        tm.config.CoinPoolAPIURL,
		DeepSeekKey:           cfg.DeepSeekKey,
		QwenKey:               cfg.QwenKey,
		GeminiAPIKey:          cfg.GeminiAPIKey,
		GeminiModel:           cfg.GeminiModel,
		CustomAPIURL:          cfg.CustomAPIURL,
		CustomAPIKey:          cfg.CustomAPIKey,
		CustomModelName:       cfg.CustomModelName,
		ScanInterval:          cfg.GetScanInterval(),
		InitialBalance:        cfg.InitialBalance,
		BTCETHLeverage:        tm.config.Leverage.BTCETHLeverage,  // 使用配置的杠杆倍数
		AltcoinLeverage:       tm.config.Leverage.AltcoinLeverage, // 使用配置的杠杆倍数
		MaxDailyLoss:          tm.config.MaxDailyLoss,
		MaxDrawdown:           tm.config.MaxDrawdown,
		StopTradingTime:       time.Duration(tm.config.StopTradingMinutes) * time.Minute,
		PromptName:            tm.config.DefaultPrompt, // 设置提示词名称
		SymbolsToAI:           cfg.SymbolsToAI, // Pass SymbolsToAI from the specific trader config
	}

	// 创建trader实例
	at, err := trader.NewAutoTrader(&traderConfig)
	if err != nil {
		return fmt.Errorf("创建trader失败: %w", err)
	}

	tm.traders[cfg.ID] = at
	log.Printf("✓ Trader '%s' (%s) 已添加", cfg.Name, cfg.AIModel)
	return nil
}

// GetTrader 获取指定ID的trader
func (tm *TraderManager) GetTrader(id string) (*trader.AutoTrader, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	t, exists := tm.traders[id]
	if !exists {
		return nil, fmt.Errorf("trader ID '%s' 不存在", id)
	}
	return t, nil
}

// GetAllTraders 获取所有trader
func (tm *TraderManager) GetAllTraders() map[string]*trader.AutoTrader {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	result := make(map[string]*trader.AutoTrader)
	for id, t := range tm.traders {
		result[id] = t
	}
	return result
}

// GetTraderIDs 获取所有trader ID列表
func (tm *TraderManager) GetTraderIDs() []string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	ids := make([]string, 0, len(tm.traders))
	for id := range tm.traders {
		ids = append(ids, id)
	}
	return ids
}

// StartAll 启动所有trader
func (tm *TraderManager) StartAll() {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	log.Println("🚀 启动所有Trader...")
	for id, t := range tm.traders {
		go func(traderID string, at *trader.AutoTrader) {
			log.Printf("▶️  启动 %s...", at.GetName())
			if err := at.Run(); err != nil {
				log.Printf("❌ %s 运行错误: %v", at.GetName(), err)
			}
		}(id, t)
	}
}

// StopAll 停止所有trader
func (tm *TraderManager) StopAll() {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	log.Println("⏹  停止所有Trader...")
	for _, t := range tm.traders {
		t.Stop()
	}
}

// GetComparisonData 获取对比数据
func (tm *TraderManager) GetComparisonData() (map[string]interface{}, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	comparison := make(map[string]interface{})
	traders := make([]map[string]interface{}, 0, len(tm.traders))

	for _, t := range tm.traders {
		account, err := t.GetAccountInfo()
		if err != nil {
			continue
		}

		status := t.GetStatus()

		traders = append(traders, map[string]interface{}{
			"trader_id":       t.GetID(),
			"trader_name":     t.GetName(),
			"ai_model":        t.GetAIModel(),
			"total_equity":    account["total_equity"],
			"total_pnl":       account["total_pnl"],
			"total_pnl_pct":   account["total_pnl_pct"],
			"position_count":  account["position_count"],
			"margin_used_pct": account["margin_used_pct"],
			"call_count":      status["call_count"],
			"is_running":      status["is_running"],
		})
	}

	comparison["traders"] = traders
	comparison["count"] = len(traders)

	return comparison, nil
}

// SetTraderPrompt sets the prompt name for a specific trader.
func (tm *TraderManager) SetTraderPrompt(traderID, promptName string) error {
	trader, err := tm.GetTrader(traderID)
	if err != nil {
		return err
	}
	trader.SetPromptName(promptName)
	log.Printf("✓ Trader '%s' prompt updated to '%s'", trader.GetName(), promptName)
	return nil
}

// StartTrader 启动指定ID的trader
func (tm *TraderManager) StartTrader(traderID string) error {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	at, exists := tm.traders[traderID]
	if !exists {
		return fmt.Errorf("trader ID '%s' 不存在", traderID)
	}

	// Use a channel to communicate the initial startup error from the goroutine
	errChan := make(chan error, 1)

	go func() {
		err := at.Run()
		if err != nil {
			errChan <- err // Send error if Run() fails
		} else {
			errChan <- nil // Send nil for success
		}
		close(errChan) // Close the channel after sending
	}()

	// Wait for the initial startup result
	select {
	case err := <-errChan:
		if err != nil {
			log.Printf("❌ Trader '%s' 启动失败: %v", at.GetName(), err)
			return fmt.Errorf("trader '%s' 启动失败: %w", at.GetName(), err)
		}
		log.Printf("▶️  Trader '%s' 已成功启动。", at.GetName())
		return nil
	case <-time.After(5 * time.Second): // Timeout for initial startup
		log.Printf("⚠️  Trader '%s' 启动超时，可能仍在后台运行。", at.GetName())
		return fmt.Errorf("trader '%s' 启动超时", at.GetName())
	}
}

// StopTrader 停止指定ID的trader
func (tm *TraderManager) StopTrader(traderID string) error {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	at, exists := tm.traders[traderID]
	if !exists {
		return fmt.Errorf("trader ID '%s' 不存在", traderID)
	}

	log.Printf("⏹️  停止 %s...", at.GetName())
	if err := at.Stop(); err != nil {
		return fmt.Errorf("停止交易员 '%s' 失败: %w", at.GetName(), err)
	}
	return nil
}

// SetDecisionMaker 设置唯一的决策者
func (tm *TraderManager) SetDecisionMaker(decisionMakerID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	log.Printf("👑 设置 '%s' 为唯一决策者...", decisionMakerID)

	found := false
	for id, t := range tm.traders {
		if id == decisionMakerID {
			t.SetDecisionMaker(true)
			log.Printf("  ✓ %s is now the decision maker.", t.GetName())
			found = true
		} else {
			t.SetDecisionMaker(false)
		}
	}

	if !found {
		return fmt.Errorf("trader ID '%s' not found", decisionMakerID)
	}

	return nil
}
