## 2025-11-04 16:50:00 - [Gemini] - 新增ai_decisions表以记录AI交互

**用户请求:**
> 检查了，但是没有记录与AI的输入和输出token

**Gemini的操作:**
*   **需求分析:** 
    *   确认了用户希望将AI的输入（Prompt）和输出（CoT、决策JSON）也存入数据库，以便于将决策过程与交易结果进行关联分析。
*   **方案设计:** 
    *   设计了新的 `ai_decisions` 表，包含 `input_prompt`, `cot_trace`, `decision_json` 等字段。
*   **代码实现:**
    *   在 `database/db.go` 中增加了创建 `ai_decisions` 表的逻辑和 `InsertAIDecision` 函数。
    *   在 `trader/auto_trader.go` 的 `runCycle` 函数末尾，增加了对 `InsertAIDecision` 的调用，将AI交互的完整上下文存入数据库。
*   **版本管理:**
    *   将所有相关修改提交到Git，并推送到远程 `feat/decision-engine-enhancements` 分支。

**变更原因/备注:**
此项修改将决策过程（AI交互）与交易结果（订单、成交）集中存储在同一个数据库中，极大地增强了数据的可分析性，为后续进行精细化的策略复盘和数据驱动的AI模型优化奠定了坚实的基础。

---