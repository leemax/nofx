## 2025-11-04 16:50:00 - [Gemini] - 新增ai_decisions表以记录AI交互

**用户请求:**
> 检查了，但是没有记录与AI的输入和输出token

**Gemini的操作:**
*   **需求分析:** 
    *   确认了用户希望将AI的输入（Prompt）和输出（CoT、决策JSON）也存入 
数据库，以便于将决策过程与交易结果进行关联分析。                        
*   **方案设计:** 
    *   设计了新的 `ai_decisions` 表，包含 `input_prompt`, `cot_trace`, 
`decision_json` 等字段。                                                
*   **代码实现:**                                                       
    *   在 `database/db.go` 中增加了创建 `ai_decisions` 表的逻辑和 `Inse
rtAIDecision` 函数。                                                    
    *   在 `trader/auto_trader.go` 的 `runCycle` 函数末尾，增加了对 `Ins
ertAIDecision` 的调用，将AI交互的完整上下文存入数据库。                 
*   **版本管理:**                                                       
    *   将所有相关修改提交到Git，并推送到远程 `feat/decision-engine-enha
nncements` 分支。                                                        
                                                                        
**变更原因/备注:**                                                      
此项修改将决策过程（AI交互）与交易结果（订单、成交）集中存储在同一个数据
库中，极大地增强了数据的可分析性，为后续进行精细化的策略复盘和数据驱动的
AI模型优化奠定了坚实的基础。                                            
                                                                        
---
## 2025-11-04 17:30:00 - [Gemini] - 在AI学习模块中显示交易手续费

**用户请求:**
> 我最想直观的看到每笔交易产生的手续费
> 你注意分析项目里有个模块“AI学习与反思”，有个子模块是呈现每笔交易信息的，我想把手续费信息直接放在这里就好了

**Gemini的操作:**
*   **需求分析:** 
    *   确认用户希望在前端“AI学习与反思”模块的交易信息展示中，集成每笔交易的手续费信息。
*   **方案设计:** 
    *   **后端:** 新增 `/api/trades` 接口，从 `nofx.db` 的 `trades` 表中查询交易记录（包含手续费）。
    *   **前端:** 修改 `AILearning.tsx` 组件，使其调用新的 `/api/trades` 接口，并用表格形式展示交易详情，包括手续费。
*   **代码实现:**
    *   在 `database/db.go` 中定义 `TradeRecord` 结构体和 `GetTrades` 函数。
    *   在 `api/server.go` 中新增 `handleTrades` 处理函数和 `/api/trades` 路由。
    *   修复了 `api/server.go` 中因缺少 `nofx/database` 导入导致的编译错误。
    *   在 `web/src/types/index.ts` 中定义 `TradeRecord` 类型。
    *   在 `web/src/lib/api.ts` 中新增 `getTrades` 函数。
    *   修改 `web/src/components/AILearning.tsx`，引入 `TradeRecord`，新增 `useSWR` hook 获取交易数据，并替换原有的“历史成交记录”部分，使用表格展示包含手续费的 `TradeRecord` 数据。
*   **版本管理:**
    *   将所有相关修改提交到Git，并推送到远程 `feat/decision-engine-enhancements` 分支。

**变更原因/备注:**
此项修改直接响应用户需求，通过在前端“AI学习与反思”模块中展示每笔交易的手续费，极大地提升了交易数据的透明度和用户对交易成本的直观理解，有助于更全面的交易分析和策略优化。

---
## 2025-11-05 10:00:00 - [Gemini] - 移动止损到盈亏平衡点功能 (Binance Only)

**用户请求:**
> 移动止损挂单，接收一个新指令，该指令用于修改一个已开仓位的止损点

**Gemini的操作:**
*   **需求分析:**
    *   用户需要实现一个“移动止损到盈亏平衡点”的功能，AI在决策时可以发出此指令。
    *   此功能目前仅针对币安交易所实现。
*   **方案设计:**
    *   **`trader/interface.go`**: 接口已包含 `SetStopLoss` 方法，可复用。
    *   **`decision/engine.go`**:
        *   修改 `Decision` 结构体，新增 `NewStopLoss float64` 字段。
        *   在 `Action` 字段中增加 `"move_sl_to_breakeven"` 类型。
        *   更新 `validateDecision` 函数，为 `"move_sl_to_breakeven"` 动作添加验证逻辑，确保 `NewStopLoss` 字段有效。
    *   **`trader/binance_futures.go`**:
        *   新增 `CancelStopLossOrders` 函数，用于取消指定币种和方向的现有止损单。
        *   修改 `SetStopLoss` 函数，使其在设置新的止损单之前，先调用 `CancelStopLossOrders` 取消旧的止损单。
    *   **`trader/auto_trader.go`**:
        *   在 `executeDecisionWithRecord` 中增加对 `"move_sl_to_breakeven"` 动作的处理。
        *   新增 `executeMoveSLToBreakevenWithRecord` 辅助函数，负责获取当前持仓信息并调用 `at.trader.SetStopLoss` 更新止损价。
*   **未实现部分:**
    *   Hyperliquid和Aster交易所的 `SetStopLoss` 实现未修改，目前不支持“移动止损到盈亏平衡点”功能。
*   **版本管理:**
    *   将所有相关修改提交到Git。

**变更原因/备注:**
此功能允许AI在持仓达到一定盈利时，自动将止损点移动到开仓价，从而保护本金，实现“不亏损的交易”。目前仅在币安交易所实现，未来可根据需求扩展到其他交易所。

---
## 2025-11-07 10:00:00 - [Gemini] - 项目分支同步策略说明

**重要提示：**

本项目的主要开发和迭代工作均在 `feat/decision-engine-enhancements` 分支上进行。

**工作流程规范：**

*   **主要开发分支：** 所有新功能开发、bug 修复和迭代都应基于本地的 `feat/decision-engine-enhancements` 分支进行。
*   **本地与远程同步：** 本地仓库应始终与远程 `origin-leemax/feat/decision-engine-enhancements` 分支保持同步。
*   **`main` 分支的使用：**
    *   `main` 分支作为项目的稳定发布分支或上游仓库的镜像。
    *   **严禁直接在 `main` 分支上进行开发、提交或进行任何直接的同步操作（如 `git merge main` 或 `git rebase main`），除非有明确的指示和批准。**
    *   任何需要从 `main` 分支获取更新的操作，都应通过 `feat/decision-engine-enhancements` 分支进行适当的合并或变基，并确保不引入冲突。
*   **代码提交方式：** 所有更改都应通过向 `feat/decision-engine-enhancements` 提交合并请求（Pull Request）的方式进行。

**变更原因/备注:**

为避免项目开发过程中因分支管理不当导致的混淆和错误，特此明确项目分支同步策略，以确保开发流程的顺畅和代码库的稳定性。