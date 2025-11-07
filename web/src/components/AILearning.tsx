import useSWR from 'swr';
import { useLanguage } from '../contexts/LanguageContext';
import { t } from '../i18n/translations';
import { api } from '../lib/api';
import { ClosedPosition } from '../types/index'; // 导入ClosedPosition类型
import { logger } from '../utils/logger';

interface TradeOutcome {
  symbol: string;
  side: string;
  quantity: number;
  leverage: number;
  open_price: number;
  close_price: number;
  position_value: number;
  margin_used: number;
  pn_l: number;
  pn_l_pct: number;
  duration: string;
  open_time: string;
  close_time: string;
  was_stop_loss: boolean;
}

interface SymbolPerformance {
  symbol: string;
  total_trades: number;
  winning_trades: number;
  losing_trades: number;
  win_rate: number;
  total_pn_l: number;
  avg_pn_l: number;
}

interface PerformanceAnalysis {
  total_trades: number;
  winning_trades: number;
  losing_trades: number;
  win_rate: number;
  avg_win: number;
  avg_loss: number;
  profit_factor: number;
  sharpe_ratio: number;
  recent_trades: TradeOutcome[]; // 保持原有类型，但不再用于渲染历史成交
  symbol_stats: { [key: string]: SymbolPerformance };
  best_symbol: string;
  worst_symbol: string;
}

interface AILearningProps {
  traderId: string;
}

export default function AILearning({ traderId }: AILearningProps) {
  const { language } = useLanguage();
  logger.log('Fetching performance data for trader:', traderId);
  const { data: performance, error } = useSWR<PerformanceAnalysis>(
    traderId ? `performance-${traderId}` : 'performance',
    async () => {
      try {
        const res = await api.getPerformance(traderId);
        logger.log('Successfully fetched performance data:', res);
        return res;
      } catch (err) {
        logger.error('Error fetching performance data:', err);
        throw err;
      }
    },
    {
      refreshInterval: 30000, // 30秒刷新（AI学习分析数据更新频率较低）
      revalidateOnFocus: false,
      dedupingInterval: 20000,
    }
  );

  // 新增：获取已平仓交易的盈亏分析
  logger.log('Fetching closed positions for trader:', traderId);
  const { data: closedPositions, error: closedPositionsError } = useSWR<ClosedPosition[]>(
    traderId ? `closed-positions-${traderId}` : 'closed-positions',
    async () => {
      try {
        const res = await api.getClosedPositions(traderId);
        logger.log('Successfully fetched closed positions:', res);
        return res;
      } catch (err) {
        logger.error('Error fetching closed positions:', err);
        throw err;
      }
    },
    {
      refreshInterval: 15000, // 15秒刷新
      revalidateOnFocus: false,
      dedupingInterval: 10000,
    }
  );

  if (error || closedPositionsError) {
    logger.error('Error in AILearning component:', error || closedPositionsError);
    return (
      <div className="rounded p-6" style={{ background: '#1E2329', border: '1px solid #2B3139' }}>
        <div style={{ color: '#F6465D' }}>{t('loadingError', language)}</div>
      </div>
    );
  }

  if (!performance || !closedPositions) {
    logger.warn('No performance or closed positions data available yet.');
    return (
      <div className="rounded p-6" style={{ background: '#1E2329', border: '1px solid #2B3139' }}>
        <div style={{ color: '#848E9C' }}>📊 {t('loading', language)}</div>
      </div>
    );
  }

  if (!performance || performance.total_trades === 0) {
    logger.warn('No complete performance data or total trades is 0.');
    return (
      <div className="rounded p-6" style={{ background: '#1E2329', border: '1px solid #2B3139' }}>
        <div className="flex items-center gap-2 mb-2">
          <span className="text-xl">🧠</span>
          <h2 className="text-lg font-bold" style={{ color: '#EAECEF' }}>{t('aiLearning', language)}</h2>
        </div>
        <div style={{ color: '#848E9C' }}>
          {t('noCompleteData', language)}
        </div>
      </div>
    );
  }

  const symbolStats = performance.symbol_stats || {};
  const symbolStatsList = Object.values(symbolStats).filter(stat => stat != null).sort(
    (a, b) => (b.total_pn_l || 0) - (a.total_pn_l || 0)
  );

  return (
    <div className="space-y-8">
      {/* 标题区 - 优化设计 */}
      <div className="relative rounded-2xl p-6 overflow-hidden" style={{
        background: 'linear-gradient(135deg, rgba(139, 92, 246, 0.15) 0%, rgba(99, 102, 241, 0.1) 50%, rgba(30, 35, 41, 0.8) 100%)',
        border: '1px solid rgba(139, 92, 246, 0.3)',
        boxShadow: '0 8px 32px rgba(139, 92, 246, 0.2)'
      }}>
        <div className="absolute top-0 right-0 w-96 h-96 rounded-full opacity-10" style={{
          background: 'radial-gradient(circle, #8B5CF6 0%, transparent 70%)',
          filter: 'blur(60px)'
        }} />
        <div className="relative flex items-center gap-4">
          <div className="w-16 h-16 rounded-2xl flex items-center justify-center text-3xl" style={{
            background: 'linear-gradient(135deg, #8B5CF6 0%, #6366F1 100%)',
            boxShadow: '0 8px 24px rgba(139, 92, 246, 0.5)',
            border: '2px solid rgba(255, 255, 255, 0.1)'
          }}>
            🧠
          </div>
          <div>
            <h2 className="text-3xl font-bold mb-1" style={{
              color: '#EAECEF',
              textShadow: '0 2px 8px rgba(139, 92, 246, 0.3)'
            }}>
              {t('aiLearning', language)}
            </h2>
            <p className="text-base" style={{ color: '#A78BFA' }}>
              {t('tradesAnalyzed', language, { count: performance.total_trades })}
            </p>
          </div>
        </div>
      </div>

      {/* 核心指标卡片 - 4列网格 */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        {/* 总交易数 */}
        <div className="rounded-2xl p-5 relative overflow-hidden group hover:scale-105 transition-transform" style={{
          background: 'linear-gradient(135deg, rgba(99, 102, 241, 0.2) 0%, rgba(30, 35, 41, 0.8) 100%)',
          border: '1px solid rgba(99, 102, 241, 0.3)',
          boxShadow: '0 4px 16px rgba(99, 102, 241, 0.2)'
        }}>
          <div className="absolute top-0 right-0 w-24 h-24 rounded-full opacity-20" style={{
            background: 'radial-gradient(circle, #6366F1 0%, transparent 70%)',
            filter: 'blur(20px)'
          }} />
          <div className="relative">
            <div className="text-xs font-semibold mb-3 uppercase tracking-wider" style={{ color: '#A5B4FC' }}>
              {t('totalTrades', language)}
            </div>
            <div className="text-4xl font-bold mono mb-1" style={{ color: '#E0E7FF' }}>
              {performance.total_trades}
            </div>
            <div className="text-xs" style={{ color: '#6366F1' }}>📊 Trades</div>
          </div>
        </div>

        {/* 胜率 */}
        <div className="rounded-2xl p-5 relative overflow-hidden group hover:scale-105 transition-transform" style={{
          background: (performance.win_rate || 0) >= 50
            ? 'linear-gradient(135deg, rgba(16, 185, 129, 0.2) 0%, rgba(30, 35, 41, 0.8) 100%)'
            : 'linear-gradient(135deg, rgba(248, 113, 113, 0.2) 0%, rgba(30, 35, 41, 0.8) 100%)',
          border: `1px solid ${(performance.win_rate || 0) >= 50 ? 'rgba(16, 185, 129, 0.4)' : 'rgba(248, 113, 113, 0.4)'}`,
          boxShadow: `0 4px 16px ${(performance.win_rate || 0) >= 50 ? 'rgba(16, 185, 129, 0.2)' : 'rgba(248, 113, 113, 0.2)'}`
        }}>
          <div className="absolute top-0 right-0 w-24 h-24 rounded-full opacity-20" style={{
            background: `radial-gradient(circle, ${(performance.win_rate || 0) >= 50 ? '#10B981' : '#F87171'} 0%, transparent 70%)`,
            filter: 'blur(20px)'
          }} />
          <div className="relative">
            <div className="text-xs font-semibold mb-3 uppercase tracking-wider" style={{
              color: (performance.win_rate || 0) >= 50 ? '#6EE7B7' : '#FCA5A5'
            }}>
              {t('winRate', language)}
            </div>
            <div className="text-4xl font-bold mono mb-1" style={{
              color: (performance.win_rate || 0) >= 50 ? '#10B981' : '#F87171'
            }}>
              {(performance.win_rate || 0).toFixed(1)}%
            </div>
            <div className="text-xs" style={{ color: '#94A3B8' }}>
              {performance.winning_trades || 0}W / {performance.losing_trades || 0}L
            </div>
          </div>
        </div>

        {/* 平均盈利 */}
        <div className="rounded-2xl p-5 relative overflow-hidden group hover:scale-105 transition-transform" style={{
          background: 'linear-gradient(135deg, rgba(14, 203, 129, 0.2) 0%, rgba(30, 35, 41, 0.8) 100%)',
          border: '1px solid rgba(14, 203, 129, 0.3)',
          boxShadow: '0 4px 16px rgba(14, 203, 129, 0.2)'
        }}>
          <div className="absolute top-0 right-0 w-24 h-24 rounded-full opacity-20" style={{
            background: 'radial-gradient(circle, #0ECB81 0%, transparent 70%)',
            filter: 'blur(20px)'
          }} />
          <div className="relative">
            <div className="text-xs font-semibold mb-3 uppercase tracking-wider" style={{ color: '#6EE7B7' }}>
              {t('avgWin', language)}
            </div>
            <div className="text-4xl font-bold mono mb-1" style={{ color: '#10B981' }}>
              +{(performance.avg_win || 0).toFixed(2)}
            </div>
            <div className="text-xs" style={{ color: '#6EE7B7' }}>📈 USDT Average</div>
          </div>
        </div>

        {/* 平均亏损 */}
        <div className="rounded-2xl p-5 relative overflow-hidden group hover:scale-105 transition-transform" style={{
          background: 'linear-gradient(135deg, rgba(246, 70, 93, 0.2) 0%, rgba(30, 35, 41, 0.8) 100%)',
          border: '1px solid rgba(246, 70, 93, 0.3)',
          boxShadow: '0 4px 16px rgba(246, 70, 93, 0.2)'
        }}>
          <div className="absolute top-0 right-0 w-24 h-24 rounded-full opacity-20" style={{
            background: 'radial-gradient(circle, #F6465D 0%, transparent 70%)',
            filter: 'blur(20px)'
          }} />
          <div className="relative">
            <div className="text-xs font-semibold mb-3 uppercase tracking-wider" style={{ color: '#FCA5A5' }}>
              {t('avgLoss', language)}
            </div>
            <div className="text-4xl font-bold mono mb-1" style={{ color: '#F87171' }}>
              {(performance.avg_loss || 0).toFixed(2)}
            </div>
            <div className="text-xs" style={{ color: '#FCA5A5' }}>📉 USDT Average</div>
          </div>
        </div>
      </div>

      {/* 关键指标：夏普比率 & 盈亏比 - 2列网格 */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* 夏普比率 */}
        <div className="rounded-2xl p-6 relative overflow-hidden" style={{
          background: 'linear-gradient(135deg, rgba(139, 92, 246, 0.25) 0%, rgba(99, 102, 241, 0.15) 50%, rgba(30, 35, 41, 0.9) 100%)',
          border: '2px solid rgba(139, 92, 246, 0.5)',
          boxShadow: '0 12px 40px rgba(139, 92, 246, 0.3)'
        }}>
          <div className="absolute top-0 right-0 w-48 h-48 rounded-full opacity-20" style={{
            background: 'radial-gradient(circle, #8B5CF6 0%, transparent 70%)',
            filter: 'blur(40px)'
          }} />
          <div className="relative">
            <div className="flex items-center gap-3 mb-4">
              <div className="w-12 h-12 rounded-xl flex items-center justify-center text-2xl" style={{
                background: 'rgba(139, 92, 246, 0.3)',
                border: '1px solid rgba(139, 92, 246, 0.5)'
              }}>
                🧬
              </div>
              <div>
                <div className="text-lg font-bold" style={{ color: '#C4B5FD' }}>夏普比率</div>
                <div className="text-xs" style={{ color: '#94A3B8' }}>风险调整后收益 · AI自我进化指标</div>
              </div>
            </div>

            <div className="flex items-end justify-between mb-4">
              <div className="text-6xl font-bold mono" style={{
                color: (performance.sharpe_ratio || 0) >= 2 ? '#10B981' :
                       (performance.sharpe_ratio || 0) >= 1 ? '#22D3EE' :
                       (performance.sharpe_ratio || 0) >= 0 ? '#F0B90B' : '#F87171',
                textShadow: '0 4px 12px rgba(0, 0, 0, 0.3)'
              }}>
                {performance.sharpe_ratio ? performance.sharpe_ratio.toFixed(2) : 'N/A'}
              </div>

              {performance.sharpe_ratio !== undefined && (
                <div className="text-right mb-2">
                  <div className="text-sm font-bold px-3 py-1 rounded-lg" style={{
                    color: (performance.sharpe_ratio || 0) >= 2 ? '#10B981' :
                           (performance.sharpe_ratio || 0) >= 1 ? '#22D3EE' :
                           (performance.sharpe_ratio || 0) >= 0 ? '#F0B90B' : '#F87171',
                    background: (performance.sharpe_ratio || 0) >= 2 ? 'rgba(16, 185, 129, 0.2)' :
                               (performance.sharpe_ratio || 0) >= 1 ? 'rgba(34, 211, 238, 0.2)' :
                               (performance.sharpe_ratio || 0) >= 0 ? 'rgba(240, 185, 11, 0.2)' : 'rgba(248, 113, 113, 0.2)'
                  }}>
                    {performance.sharpe_ratio >= 2 ? '🟢 卓越表现' :
                     performance.sharpe_ratio >= 1 ? '🟢 良好表现' :
                     performance.sharpe_ratio >= 0 ? '🟡 波动较大' : '🔴 需要调整'}
                  </div>
                </div>
              )}
            </div>

            {performance.sharpe_ratio !== undefined && (
              <div className="rounded-xl p-4" style={{
                background: 'rgba(0, 0, 0, 0.4)',
                border: '1px solid rgba(139, 92, 246, 0.3)'
              }}>
                <div className="text-sm leading-relaxed" style={{ color: '#DDD6FE' }}>
                  {performance.sharpe_ratio >= 2 && '✨ AI策略非常有效！风险调整后收益优异，可适度扩大仓位但保持纪律。'}
                  {performance.sharpe_ratio >= 1 && performance.sharpe_ratio < 2 && '✅ 策略表现稳健，风险收益平衡良好，继续保持当前策略。'}
                  {performance.sharpe_ratio >= 0 && performance.sharpe_ratio < 1 && '⚠️ 收益为正但波动较大，AI正在优化策略，降低风险。'}
                  {performance.sharpe_ratio < 0 && '🚨 当前策略需要调整！AI已自动进入保守模式，减少仓位和交易频率。'}
                </div>
              </div>
            )}
          </div>
        </div>

        {/* 盈亏比 */}
        <div className="rounded-2xl p-6 relative overflow-hidden" style={{
          background: 'linear-gradient(135deg, rgba(240, 185, 11, 0.25) 0%, rgba(252, 213, 53, 0.15) 50%, rgba(30, 35, 41, 0.9) 100%)',
          border: '2px solid rgba(240, 185, 11, 0.5)',
          boxShadow: '0 12px 40px rgba(240, 185, 11, 0.3)'
        }}>
          <div className="absolute top-0 right-0 w-48 h-48 rounded-full opacity-20" style={{
            background: 'radial-gradient(circle, #F0B90B 0%, transparent 70%)',
            filter: 'blur(40px)'
          }} />
          <div className="relative">
            <div className="flex items-center gap-3 mb-4">
              <div className="w-12 h-12 rounded-xl flex items-center justify-center text-2xl" style={{
                background: 'rgba(240, 185, 11, 0.3)',
                border: '1px solid rgba(240, 185, 11, 0.5)'
              }}>
                💰
              </div>
              <div>
                <div className="text-lg font-bold" style={{ color: '#FCD34D' }}>
                  {t('profitFactor', language)}
                </div>
                <div className="text-xs" style={{ color: '#94A3B8' }}>
                  {t('avgWinDivLoss', language)}
                </div>
              </div>
            </div>

            <div className="flex items-end justify-between mb-4">
              <div className="text-6xl font-bold mono" style={{
                color: (performance.profit_factor || 0) >= 2.0 ? '#10B981' :
                       (performance.profit_factor || 0) >= 1.5 ? '#F0B90B' :
                       (performance.profit_factor || 0) >= 1.0 ? '#FB923C' : '#F87171',
                textShadow: '0 4px 12px rgba(0, 0, 0, 0.3)'
              }}>
                {(performance.profit_factor || 0) > 0 ? (performance.profit_factor || 0).toFixed(2) : 'N/A'}
              </div>

              <div className="text-right mb-2">
                <div className="text-sm font-bold px-3 py-1 rounded-lg" style={{
                  color: (performance.profit_factor || 0) >= 2.0 ? '#10B981' :
                         (performance.profit_factor || 0) >= 1.5 ? '#F0B90B' : '#94A3B8',
                  background: (performance.profit_factor || 0) >= 2.0 ? 'rgba(16, 185, 129, 0.2)' :
                             (performance.profit_factor || 0) >= 1.5 ? 'rgba(240, 185, 11, 0.2)' : 'rgba(148, 163, 184, 0.2)'
                }}>
                  {(performance.profit_factor || 0) >= 2.0 && t('excellent', language)}
                  {(performance.profit_factor || 0) >= 1.5 && (performance.profit_factor || 0) < 2.0 && t('good', language)}
                  {(performance.profit_factor || 0) >= 1.0 && (performance.profit_factor || 0) < 1.5 && t('fair', language)}
                  {(performance.profit_factor || 0) > 0 && (performance.profit_factor || 0) < 1.0 && t('poor', language)}
                </div>
              </div>
            </div>

            <div className="rounded-xl p-4" style={{
              background: 'rgba(0, 0, 0, 0.4)',
              border: '1px solid rgba(240, 185, 11, 0.3)'
            }}>
              <div className="text-sm leading-relaxed" style={{ color: '#FEF3C7' }}>
                {(performance.profit_factor || 0) >= 2.0 && '🔥 盈利能力出色！每亏1元能赚' + (performance.profit_factor || 0).toFixed(1) + '元，AI策略表现优异。'}
                {(performance.profit_factor || 0) >= 1.5 && (performance.profit_factor || 0) < 2.0 && '✓ 策略稳定盈利，盈亏比健康，继续保持纪律性交易。'}
                {(performance.profit_factor || 0) >= 1.0 && (performance.profit_factor || 0) < 1.5 && '⚠️ 策略略有盈利但需优化，AI正在调整仓位和止损策略。'}
                {(performance.profit_factor || 0) > 0 && (performance.profit_factor || 0) < 1.0 && '❌ 平均亏损大于盈利，需要调整策略或降低交易频率。'}
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* 最佳/最差币种 - 独立行 */}
      {(performance.best_symbol || performance.worst_symbol) && (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          {performance.best_symbol && (
            <div className="rounded-2xl p-6 backdrop-blur-sm" style={{
              background: 'linear-gradient(135deg, rgba(16, 185, 129, 0.15) 0%, rgba(14, 203, 129, 0.05) 100%)',
              border: '1px solid rgba(16, 185, 129, 0.3)',
              boxShadow: '0 4px 16px rgba(16, 185, 129, 0.1)'
            }}>
              <div className="flex items-center gap-2 mb-3">
                <span className="text-2xl">🏆</span>
                <span className="text-sm font-semibold" style={{ color: '#6EE7B7' }}>{t('bestPerformer', language)}</span>
              </div>
              <div className="text-3xl font-bold mono mb-1" style={{ color: '#10B981' }}>
                {performance.best_symbol}
              </div>
              {symbolStats[performance.best_symbol] && (
                <div className="text-lg font-semibold" style={{ color: '#6EE7B7' }}>
                  {symbolStats[performance.best_symbol].total_pn_l > 0 ? '+' : ''}
                  {symbolStats[performance.best_symbol].total_pn_l.toFixed(2)} USDT {t('pnl', language)}
                </div>
              )}
            </div>
          )}

          {performance.worst_symbol && (
            <div className="rounded-2xl p-6 backdrop-blur-sm" style={{
              background: 'linear-gradient(135deg, rgba(248, 113, 113, 0.15) 0%, rgba(246, 70, 93, 0.05) 100%)',
              border: '1px solid rgba(248, 113, 113, 0.3)',
              boxShadow: '0 4px 16px rgba(248, 113, 113, 0.1)'
            }}>
              <div className="flex items-center gap-2 mb-3">
                <span className="text-2xl">📉</span>
                <span className="text-sm font-semibold" style={{ color: '#FCA5A5' }}>{t('worstPerformer', language)}</span>
              </div>
              <div className="text-3xl font-bold mono mb-1" style={{ color: '#F87171' }}>
                {performance.worst_symbol}
              </div>
              {symbolStats[performance.worst_symbol] && (
                <div className="text-lg font-semibold" style={{ color: '#FCA5A5' }}>
                  {symbolStats[performance.worst_symbol].total_pn_l > 0 ? '+' : ''}
                  {symbolStats[performance.worst_symbol].total_pn_l.toFixed(2)} USDT {t('pnl', language)}
                </div>
              )}
            </div>
          )}
        </div>
      )}

      {/* 币种表现 & 历史成交 - 左右分屏 2列布局 */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* 左侧：币种表现统计表格 */}
        {symbolStatsList.length > 0 && (
          <div className="rounded-2xl overflow-hidden" style={{
            background: 'rgba(30, 35, 41, 0.4)',
            border: '1px solid rgba(99, 102, 241, 0.2)',
            boxShadow: '0 4px 16px rgba(0, 0, 0, 0.2)',
            maxHeight: 'calc(100vh - 200px)'
          }}>
            <div className="p-5 border-b sticky top-0 z-10" style={{
              borderColor: 'rgba(99, 102, 241, 0.2)',
              background: 'rgba(30, 35, 41, 0.95)',
              backdropFilter: 'blur(10px)'
            }}>
              <h3 className="font-bold flex items-center gap-2 text-lg" style={{ color: '#E0E7FF' }}>
                📊 {t('symbolPerformance', language)}
              </h3>
            </div>
            <div className="overflow-y-auto" style={{ maxHeight: 'calc(100vh - 280px)' }}>
              <table className="w-full">
                <thead className="sticky top-0 z-10">
                  <tr style={{ background: 'rgba(15, 23, 42, 0.95)', backdropFilter: 'blur(10px)' }}>
                    <th className="text-left px-4 py-3 text-xs font-semibold" style={{ color: '#94A3B8' }}>Symbol</th>
                    <th className="text-right px-4 py-3 text-xs font-semibold" style={{ color: '#94A3B8' }}>Trades</th>
                    <th className="text-right px-4 py-3 text-xs font-semibold" style={{ color: '#94A3B8' }}>Win Rate</th>
                    <th className="text-right px-4 py-3 text-xs font-semibold" style={{ color: '#94A3B8' }}>Total P&L (USDT)</th>
                    <th className="text-right px-4 py-3 text-xs font-semibold" style={{ color: '#94A3B8' }}>Avg P&L (USDT)</th>
                  </tr>
                </thead>
                <tbody>
                  {symbolStatsList.map((stat, idx) => (
                    <tr key={stat.symbol} className="transition-colors hover:bg-white/5" style={{
                      borderTop: idx > 0 ? '1px solid rgba(99, 102, 241, 0.1)' : 'none'
                    }}>
                      <td className="px-4 py-3">
                        <span className="font-bold mono text-sm" style={{ color: '#E0E7FF' }}>{stat.symbol}</span>
                      </td>
                      <td className="px-4 py-3 text-right mono text-sm" style={{ color: '#CBD5E1' }}>
                        {stat.total_trades}
                      </td>
                      <td className="px-4 py-3 text-right mono text-sm font-semibold" style={{
                        color: (stat.win_rate || 0) >= 50 ? '#10B981' : '#F87171'
                      }}>
                        {(stat.win_rate || 0).toFixed(1)}%
                      </td>
                      <td className="px-4 py-3 text-right mono text-sm font-bold" style={{
                        color: (stat.total_pn_l || 0) > 0 ? '#10B981' : '#F87171'
                      }}>
                        {(stat.total_pn_l || 0) > 0 ? '+' : ''}{(stat.total_pn_l || 0).toFixed(2)}
                      </td>
                      <td className="px-4 py-3 text-right mono text-sm" style={{
                        color: (stat.avg_pn_l || 0) > 0 ? '#10B981' : '#F87171'
                      }}>
                        {(stat.avg_pn_l || 0) > 0 ? '+' : ''}{(stat.avg_pn_l || 0).toFixed(2)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}

        {/* 右侧：已平仓交易分析 */}
        <div className="rounded-2xl overflow-hidden" style={{
          background: 'rgba(30, 35, 41, 0.4)',
          border: '1px solid rgba(240, 185, 11, 0.2)',
          maxHeight: 'calc(100vh - 200px)'
        }}>
          <div className="p-5 border-b sticky top-0 z-10" style={{
            background: 'rgba(240, 185, 11, 0.1)',
            borderColor: 'rgba(240, 185, 11, 0.3)',
            backdropFilter: 'blur(10px)'
          }}>
            <div className="flex items-center gap-2">
              <span className="text-2xl">🧾</span>
              <div>
                <h3 className="font-bold text-lg" style={{ color: '#FCD34D' }}>{t('closedTrades', language)}</h3>
                <p className="text-xs" style={{ color: '#94A3B8' }}>
                  {closedPositions && closedPositions.length > 0
                    ? t('closedTradesAnalyzed', language, { count: closedPositions.length })
                    : t('closedTradesWillAppear', language)}
                </p>
              </div>
            </div>
          </div>

          <div className="overflow-y-auto" style={{ maxHeight: 'calc(100vh - 280px)' }}>
            {closedPositions && closedPositions.length > 0 ? (
              <table className="w-full">
                <thead className="sticky top-0 z-10">
                  <tr style={{ background: 'rgba(15, 23, 42, 0.95)', backdropFilter: 'blur(10px)' }}>
                    <th className="text-left px-4 py-3 text-xs font-semibold" style={{ color: '#94A3B8' }}>Symbol</th>
                    <th className="text-right px-4 py-3 text-xs font-semibold" style={{ color: '#94A3B8' }}>Net Profit</th>
                    <th className="text-right px-4 py-3 text-xs font-semibold" style={{ color: '#94A3B8' }}>Total Fees</th>
                    <th className="text-right px-4 py-3 text-xs font-semibold" style={{ color: '#94A3B8' }}>Duration</th>
                    <th className="text-right px-4 py-3 text-xs font-semibold" style={{ color: '#94A3B8' }}>Avg Open</th>
                    <th className="text-right px-4 py-3 text-xs font-semibold" style={{ color: '#94A3B8' }}>Avg Close</th>
                    <th className="text-right px-4 py-3 text-xs font-semibold" style={{ color: '#94A3B8' }}>Quantity</th>
                    <th className="text-right px-4 py-3 text-xs font-semibold" style={{ color: '#94A3B8' }}>Open Time</th>
                    <th className="text-right px-4 py-3 text-xs font-semibold" style={{ color: '#94A3B8' }}>Close Time</th>
                  </tr>
                </thead>
                <tbody>
                  {closedPositions.map((pos: ClosedPosition, index: number) => (
                    <tr key={index} className="transition-colors hover:bg-white/5" style={{
                      borderTop: '1px solid rgba(240, 185, 11, 0.1)'
                    }}>
                      <td className="px-4 py-3">
                        <span className="font-bold mono text-sm" style={{ color: '#E0E7FF' }}>{pos.symbol}</span>
                      </td>
                      <td className="px-4 py-3 text-right mono text-sm font-bold" style={{
                        color: pos.net_profit > 0 ? '#10B981' : '#F87171'
                      }}>
                        {pos.net_profit > 0 ? '+' : ''}{pos.net_profit.toFixed(4)}
                      </td>
                      <td className="px-4 py-3 text-right mono text-sm" style={{ color: '#FCD34D' }}>
                        {pos.total_fees.toFixed(6)}
                      </td>
                      <td className="px-4 py-3 text-right text-xs" style={{ color: '#94A3B8' }}>
                        {formatDuration(pos.duration)}
                      </td>
                      <td className="px-4 py-3 text-right mono text-sm" style={{ color: '#CBD5E1' }}>
                        {pos.avg_open_price.toFixed(4)}
                      </td>
                      <td className="px-4 py-3 text-right mono text-sm" style={{ color: '#CBD5E1' }}>
                        {pos.avg_close_price.toFixed(4)}
                      </td>
                      <td className="px-4 py-3 text-right mono text-sm" style={{ color: '#CBD5E1' }}>
                        {pos.total_quantity.toFixed(4)}
                      </td>
                      <td className="px-4 py-3 text-right text-xs" style={{ color: '#94A3B8' }}>
                        {formatTimestamp(pos.open_time)}
                      </td>
                      <td className="px-4 py-3 text-right text-xs" style={{ color: '#94A3B8' }}>
                        {formatTimestamp(pos.close_time)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            ) : (
              <div className="p-6 text-center">
                <div className="text-4xl mb-2 opacity-50">🧾</div>
                <div style={{ color: '#94A3B8' }}>{t('noClosedTrades', language)}</div>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* AI学习说明 - 现代化设计 */}
      <div className="rounded-2xl p-6 backdrop-blur-sm" style={{
        background: 'linear-gradient(135deg, rgba(240, 185, 11, 0.1) 0%, rgba(252, 213, 53, 0.05) 100%)',
        border: '1px solid rgba(240, 185, 11, 0.2)',
        boxShadow: '0 4px 16px rgba(240, 185, 11, 0.1)'
      }}>
        <div className="flex items-start gap-4">
          <div className="w-10 h-10 rounded-lg flex items-center justify-center text-xl flex-shrink-0" style={{
            background: 'rgba(240, 185, 11, 0.2)',
            border: '1px solid rgba(240, 185, 11, 0.3)'
          }}>
            💡
          </div>
          <div>
            <h3 className="font-bold mb-3 text-base" style={{ color: '#FCD34D' }}>{t('howAILearns', language)}</h3>
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 text-sm">
              <div className="flex items-start gap-2">
                <span style={{ color: '#F0B90B' }}>•</span>
                <span style={{ color: '#CBD5E1' }}>{t('aiLearningPoint1', language)}</span>
              </div>
              <div className="flex items-start gap-2">
                <span style={{ color: '#F0B90B' }}>•</span>
                <span style={{ color: '#CBD5E1' }}>{t('aiLearningPoint2', language)}</span>
              </div>
              <div className="flex items-start gap-2">
                <span style={{ color: '#F0B90B' }}>•</span>
                <span style={{ color: '#CBD5E1' }}>{t('aiLearningPoint3', language)}</span>
              </div>
              <div className="flex items-start gap-2">
                <span style={{ color: '#F0B90B' }}>•</span>
                <span style={{ color: '#CBD5E1' }}>{t('aiLearningPoint4', language)}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

// 格式化持仓时长
function formatDuration(duration: string | undefined): string {
  if (!duration) return '-';

  // 尝试匹配 "XhYmZs" 格式
  const hmsMatch = duration.match(/(\d+)h(\d+)m(\d+\.?\d*)s/);
  if (hmsMatch) {
    const [, h, m, s] = hmsMatch;
    let result = '';
    if (h && h !== '0') result += `${h}h `;
    if (m && m !== '0') result += `${m}m `;
    if (s && parseFloat(s) > 0) result += `${parseFloat(s).toFixed(0)}s`;
    return result.trim() || '0s';
  }

  // 尝试匹配纯秒数
  const secondsMatch = duration.match(/^(\d+\.?\d*)s$/);
  if (secondsMatch) {
    const totalSeconds = parseFloat(secondsMatch[1]);
    if (totalSeconds < 60) return `${totalSeconds.toFixed(0)}s`;
    const hours = Math.floor(totalSeconds / 3600);
    const minutes = Math.floor((totalSeconds % 3600) / 60);
    const seconds = totalSeconds % 60;
    let result = '';
    if (hours > 0) result += `${hours}h `;
    if (minutes > 0) result += `${minutes}m `;
    if (seconds > 0) result += `${seconds.toFixed(0)}s`;
    return result.trim();
  }
  
  return duration; // Fallback
}

// 格式化时间戳
function formatTimestamp(timestamp: string | undefined): string {
  if (!timestamp) return '-';
  try {
    const date = new Date(timestamp);
    // 获取UTC时间组件
    const year = date.getUTCFullYear();
    const month = date.getUTCMonth() + 1; // 月份从0开始
    const day = date.getUTCDate();
    const hours = date.getUTCHours();
    const minutes = date.getUTCMinutes();
    const seconds = date.getUTCSeconds();

    // 转换为UTC+8
    const bzDate = new Date(Date.UTC(year, month - 1, day, hours + 8, minutes, seconds));

    const bzYear = bzDate.getFullYear();
    const bzMonth = (bzDate.getMonth() + 1).toString().padStart(2, '0');
    const bzDay = bzDate.getDate().toString().padStart(2, '0');
    const bzHours = bzDate.getHours().toString().padStart(2, '0');
    const bzMinutes = bzDate.getMinutes().toString().padStart(2, '0');
    const bzSeconds = bzDate.getSeconds().toString().padStart(2, '0');

    return `${bzYear}-${bzMonth}-${bzDay} ${bzHours}:${bzMinutes}:${bzSeconds}`;
  } catch (e) {
    return timestamp; // 如果格式化失败，返回原始字符串
  }
}
