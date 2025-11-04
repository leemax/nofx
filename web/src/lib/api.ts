import type {
  SystemStatus,
  AccountInfo,
  Position,
  DecisionRecord,
  Statistics,
  TraderInfo,
  CompetitionData,
  TradeRecord,
  ClosedPosition,
} from '../types';

const API_BASE = '/api';

const fetchOptions = {
  cache: 'no-store' as RequestCache,
  headers: {
    'Cache-Control': 'no-cache',
  },
};

export const api = {
  // 竞赛相关接口
  async getCompetition(): Promise<CompetitionData> {
    const res = await fetch(`${API_BASE}/competition`, fetchOptions);
    if (!res.ok) throw new Error('获取竞赛数据失败');
    return res.json();
  },

  async getTraders(): Promise<TraderInfo[]> {
    const res = await fetch(`${API_BASE}/traders`, fetchOptions);
    if (!res.ok) throw new Error('获取trader列表失败');
    return res.json();
  },

  // 获取系统状态（支持trader_id）
  async getStatus(traderId?: string): Promise<SystemStatus> {
    const url = traderId
      ? `${API_BASE}/status?trader_id=${traderId}`
      : `${API_BASE}/status`;
    const res = await fetch(url, fetchOptions);
    if (!res.ok) throw new Error('获取系统状态失败');
    return res.json();
  },

  // 获取账户信息（支持trader_id）
  async getAccount(traderId?: string): Promise<AccountInfo> {
    const url = traderId
      ? `${API_BASE}/account?trader_id=${traderId}`
      : `${API_BASE}/account`;
    const res = await fetch(url, fetchOptions);
    if (!res.ok) throw new Error('获取账户信息失败');
    const data = await res.json();
    console.log('Account data fetched:', data);
    return data;
  },

  // 获取持仓列表（支持trader_id）
  async getPositions(traderId?: string): Promise<Position[]> {
    const url = traderId
      ? `${API_BASE}/positions?trader_id=${traderId}`
      : `${API_BASE}/positions`;
    const res = await fetch(url, fetchOptions);
    if (!res.ok) throw new Error('获取持仓列表失败');
    return res.json();
  },

  // 获取决策日志（支持trader_id）
  async getDecisions(traderId?: string): Promise<DecisionRecord[]> {
    const url = traderId
      ? `${API_BASE}/decisions?trader_id=${traderId}`
      : `${API_BASE}/decisions`;
    const res = await fetch(url, fetchOptions);
    if (!res.ok) throw new Error('获取决策日志失败');
    return res.json();
  },

  // 获取最新决策（支持trader_id）
  async getLatestDecisions(traderId?: string): Promise<DecisionRecord[]> {
    const url = traderId
      ? `${API_BASE}/decisions/latest?trader_id=${traderId}`
      : `${API_BASE}/decisions/latest`;
    const res = await fetch(url, fetchOptions);
    if (!res.ok) throw new Error('获取最新决策失败');
    return res.json();
  },

  // 获取统计信息（支持trader_id）
  async getStatistics(traderId?: string): Promise<Statistics> {
    const url = traderId
      ? `${API_BASE}/statistics?trader_id=${traderId}`
      : `${API_BASE}/statistics`;
    const res = await fetch(url, fetchOptions);
    if (!res.ok) throw new Error('获取统计信息失败');
    return res.json();
  },

  // 获取收益率历史数据（支持trader_id）
  async getEquityHistory(traderId?: string): Promise<any[]> {
    const url = traderId
      ? `${API_BASE}/equity-history?trader_id=${traderId}`
      : `${API_BASE}/equity-history`;
    const res = await fetch(url, fetchOptions);
    if (!res.ok) throw new Error('获取历史数据失败');
    return res.json();
  },

  // 获取AI学习表现分析（支持trader_id）
  async getPerformance(traderId?: string): Promise<any> {
    const url = traderId
      ? `${API_BASE}/performance?trader_id=${traderId}`
      : `${API_BASE}/performance`;
    const res = await fetch(url, fetchOptions);
    if (!res.ok) throw new Error('获取AI学习数据失败');
    return res.json();
  },

  // 获取交易记录（支持trader_id）
  async getTrades(traderId?: string): Promise<TradeRecord[]> {
    const url = traderId
      ? `${API_BASE}/trades?trader_id=${traderId}`
      : `${API_BASE}/trades`;
    const res = await fetch(url, fetchOptions);
    if (!res.ok) throw new Error('获取交易记录失败');
    return res.json();
  },

  // 获取已平仓交易的盈亏分析（支持trader_id）
  async getClosedPositions(traderId?: string): Promise<ClosedPosition[]> {
    const url = traderId
      ? `${API_BASE}/closed-positions?trader_id=${traderId}`
      : `${API_BASE}/closed-positions`;
    const res = await fetch(url, fetchOptions);
    if (!res.ok) throw new Error('获取已平仓交易失败');
    return res.json();
  },

  // 获取系统提示词列表
  async getPrompts(): Promise<PromptsResponse> {
    const res = await fetch(`${API_BASE}/prompts`, fetchOptions);
    if (!res.ok) throw new Error('获取系统提示词列表失败');
    return res.json();
  },

  // 手动触发决策
  async forceDecision(traderId: string): Promise<any> {
    const url = `${API_BASE}/force-decision?trader_id=${traderId}`;
    const res = await fetch(url, {
      method: 'POST',
      ...fetchOptions,
    });
    if (!res.ok) throw new Error('手动触发决策失败');
    return res.json();
  },

  // 设置trader的默认提示词
  async setTraderPrompt(traderId: string, promptName: string): Promise<any> {
    const url = `${API_BASE}/trader/prompt`;
    const res = await fetch(url, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ trader_id: traderId, prompt_name: promptName }),
    });
    if (!res.ok) throw new Error('设置默认提示词失败');
    return res.json();
  },
};
