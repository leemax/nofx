package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3" // 导入SQLite驱动
)

var db *sql.DB

// InitDB 初始化数据库连接并创建表
func InitDB(filepath string) error {
	var err error
	// 连接数据库
	db, err = sql.Open("sqlite3", filepath)
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}

	// 强制使用单个连接，避免并发写入时的 "database is locked" 错误
	db.SetMaxOpenConns(1)

	log.Printf("✓ 数据库连接成功: %s", filepath)

	// 创建表
	if err := createTables(); err != nil {
		return fmt.Errorf("创建表失败: %w", err)
	}

	return nil
}

// createTables 创建数据库表
func createTables() error {
	// 订单表
	ordersTableSQL := `
	CREATE TABLE IF NOT EXISTS orders (
		order_id INTEGER PRIMARY KEY,
		trader_id TEXT NOT NULL,
		symbol TEXT NOT NULL,
		side TEXT NOT NULL,
		type TEXT NOT NULL,
		status TEXT NOT NULL,
		price REAL NOT NULL,
		quantity REAL NOT NULL,
		created_at DATETIME NOT NULL
	);
	`
	// 成交表
	tradesTableSQL := `
	CREATE TABLE IF NOT EXISTS trades (
		trade_id INTEGER PRIMARY KEY,
		order_id INTEGER NOT NULL,
		trader_id TEXT NOT NULL,
		symbol TEXT NOT NULL,
		price REAL NOT NULL,
		quantity REAL NOT NULL,
		commission REAL NOT NULL,
		commission_asset TEXT NOT NULL,
		is_buyer BOOLEAN NOT NULL,
		is_maker BOOLEAN NOT NULL,
		timestamp DATETIME NOT NULL,
		FOREIGN KEY (order_id) REFERENCES orders (order_id)
	);
	`
	// 账户快照表
	accountSnapshotsTableSQL := `
	CREATE TABLE IF NOT EXISTS account_snapshots (
		snapshot_id INTEGER PRIMARY KEY AUTOINCREMENT,
		trader_id TEXT NOT NULL,
		timestamp DATETIME NOT NULL,
		total_equity REAL NOT NULL,
		available_balance REAL NOT NULL,
		total_pnl_pct REAL NOT NULL,
		margin_used_pct REAL NOT NULL
	);
	`

	// AI决策记录表
	aiDecisionsTableSQL := `
	CREATE TABLE IF NOT EXISTS ai_decisions (
		decision_id INTEGER PRIMARY KEY AUTOINCREMENT,
		trader_id TEXT NOT NULL,
		cycle_number INTEGER NOT NULL,
		timestamp DATETIME NOT NULL,
		input_prompt TEXT,
		cot_trace TEXT,
		decision_json TEXT,
		error_message TEXT
	);
	`

	// 执行SQL
	for _, tableSQL := range []string{ordersTableSQL, tradesTableSQL, accountSnapshotsTableSQL, aiDecisionsTableSQL} {
		_, err := db.Exec(tableSQL)
		if err != nil {
			return err
		}
	}

	log.Println("✓ 数据库表结构检查/创建完成")
	return nil
}

// InsertAIDecision 插入一条AI决策记录
func InsertAIDecision(traderID string, cycleNumber int, timestamp time.Time, inputPrompt, cotTrace, decisionJSON, errorMessage string) error {
	query := `INSERT INTO ai_decisions (trader_id, cycle_number, timestamp, input_prompt, cot_trace, decision_json, error_message) VALUES (?, ?, ?, ?, ?, ?, ?)`
	stmt, err := db.Prepare(query)
	if err != nil {
		return fmt.Errorf("准备AI决策插入SQL失败: %w", err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(traderID, cycleNumber, timestamp, inputPrompt, cotTrace, decisionJSON, errorMessage)
	if err != nil {
		return fmt.Errorf("插入AI决策记录失败: %w", err)
	}

	return nil
}

// InsertAccountSnapshot 插入一条账户快照记录
func InsertAccountSnapshot(traderID string, equity, available, pnlPct, marginPct float64) error {
	query := `INSERT INTO account_snapshots (trader_id, timestamp, total_equity, available_balance, total_pnl_pct, margin_used_pct) VALUES (?, ?, ?, ?, ?, ?)`
	stmt, err := db.Prepare(query)
	if err != nil {
		return fmt.Errorf("准备SQL失败: %w", err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(traderID, time.Now(), equity, available, pnlPct, marginPct)
	if err != nil {
		return fmt.Errorf("插入账户快照失败: %w", err)
	}

	return nil
}

// InsertOrder 插入一条订单记录
func InsertOrder(orderID int64, traderID, symbol, side, orderType, status string, price, quantity float64, createdAt time.Time) error {
	query := `INSERT INTO orders (order_id, trader_id, symbol, side, type, status, price, quantity, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	stmt, err := db.Prepare(query)
	if err != nil {
		return fmt.Errorf("准备订单插入SQL失败: %w", err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(orderID, traderID, symbol, side, orderType, status, price, quantity, createdAt)
	if err != nil {
		return fmt.Errorf("插入订单记录失败: %w", err)
	}

	return nil
}

// UpdateOrderStatus 更新订单状态
func UpdateOrderStatus(orderID int64, status string) error {
	query := `UPDATE orders SET status = ? WHERE order_id = ?`
	stmt, err := db.Prepare(query)
	if err != nil {
		return fmt.Errorf("准备订单状态更新SQL失败: %w", err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(status, orderID)
	if err != nil {
		return fmt.Errorf("更新订单状态失败: %w", err)
	}

	return nil
}

// InsertTrade 插入一条成交记录
func InsertTrade(tradeID, orderID int64, traderID, symbol, commissionAsset string, price, quantity, commission float64, isBuyer, isMaker bool, timestamp time.Time) error {
	query := `INSERT OR IGNORE INTO trades (trade_id, order_id, trader_id, symbol, price, quantity, commission, commission_asset, is_buyer, is_maker, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	stmt, err := db.Prepare(query)
	if err != nil {
		return fmt.Errorf("准备成交插入SQL失败: %w", err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(tradeID, orderID, traderID, symbol, price, quantity, commission, commissionAsset, isBuyer, isMaker, timestamp)
	if err != nil {
		return fmt.Errorf("插入成交记录失败: %w", err)
	}

	return nil
}

// TradeRecord 映射trades表的一条记录
type TradeRecord struct {
	TradeID         int64     `json:"trade_id"`
	OrderID         int64     `json:"order_id"`
	TraderID        string    `json:"trader_id"`
	Symbol          string    `json:"symbol"`
	Price           float64   `json:"price"`
	Quantity        float64   `json:"quantity"`
	Commission      float64   `json:"commission"`
	CommissionAsset string    `json:"commission_asset"`
	IsBuyer         bool      `json:"is_buyer"`
	IsMaker         bool      `json:"is_maker"`
	Timestamp       time.Time `json:"timestamp"`
}

// GetTrades 从trades表中获取指定traderID的交易记录
func GetTrades(traderID string) ([]TradeRecord, error) {
	query := `SELECT trade_id, order_id, trader_id, symbol, price, quantity, commission, commission_asset, is_buyer, is_maker, timestamp FROM trades WHERE trader_id = ? ORDER BY timestamp DESC`
	rows, err := db.Query(query, traderID)
	if err != nil {
		return nil, fmt.Errorf("查询交易记录失败: %w", err)
	}
	defer rows.Close()

	var trades []TradeRecord
	for rows.Next() {
		var trade TradeRecord
		if err := rows.Scan(&trade.TradeID, &trade.OrderID, &trade.TraderID, &trade.Symbol, &trade.Price, &trade.Quantity, &trade.Commission, &trade.CommissionAsset, &trade.IsBuyer, &trade.IsMaker, &trade.Timestamp); err != nil {
			log.Printf("❌ 扫描交易记录失败: %v", err)
			continue
		}
		trades = append(trades, trade)
	}

	return trades, nil
}


// AIDecisionRecord 映射ai_decisions表的一条记录
type AIDecisionRecord struct {
	DecisionID   int64     `json:"decision_id"`
	TraderID     string    `json:"trader_id"`
	CycleNumber  int       `json:"cycle_number"`
	Timestamp    time.Time `json:"timestamp"`
	InputPrompt  string    `json:"input_prompt"`
	CoTTrace     string    `json:"cot_trace"`
	DecisionJSON string    `json:"decision_json"`
	ErrorMessage string    `json:"error_message"`
}

// GetAIDecisionsByTraderID 从ai_decisions表中获取指定traderID的AI决策记录
func GetAIDecisionsByTraderID(traderID string) ([]*AIDecisionRecord, error) {
	query := `SELECT decision_id, trader_id, cycle_number, timestamp, input_prompt, cot_trace, decision_json, error_message FROM ai_decisions WHERE trader_id = ? ORDER BY timestamp ASC`
	rows, err := db.Query(query, traderID)
	if err != nil {
		return nil, fmt.Errorf("查询AI决策记录失败: %w", err)
	}
	defer rows.Close()

	var records []*AIDecisionRecord
	for rows.Next() {
		var record AIDecisionRecord
		if err := rows.Scan(&record.DecisionID, &record.TraderID, &record.CycleNumber, &record.Timestamp, &record.InputPrompt, &record.CoTTrace, &record.DecisionJSON, &record.ErrorMessage); err != nil {
			log.Printf("❌ 扫描AI决策记录失败: %v", err)
			continue
		}
		records = append(records, &record)
	}

	return records, nil
}