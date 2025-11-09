import sqlite3
import pandas as pd
import json
from datetime import datetime, timedelta

# --- Configuration ---
DB_FILE = "nofx_副本.db"
TRADER_ID = "binance_geminiFlash_eth_focused" # <-- IMPORTANT: Change this to the trader_id you want to analyze
NUM_TRADES_TO_SHOW = 20
NUM_DECISIONS_TO_SHOW = 10
ANALYSIS_PERIOD_HOURS = 72 # Analyze trades and decisions from the last 72 hours

def get_trade_data(db_conn, trader_id):
    """Fetches trade data from the database."""
    print(f"Fetching trades for trader_id: {trader_id}")
    try:
        # Filter for trades within the analysis period
        start_time = datetime.now() - timedelta(hours=ANALYSIS_PERIOD_HOURS)
        df = pd.read_sql_query(f"SELECT * FROM trades WHERE trader_id = '{trader_id}' AND timestamp >= '{start_time.isoformat()}' ORDER BY timestamp ASC", db_conn)
        df['timestamp'] = pd.to_datetime(df['timestamp'])
        return df
    except pd.io.sql.DatabaseError as e:
        print(f"Error fetching trades: {e}")
        print("It's possible the 'trades' table doesn't exist or is empty.")
        return pd.DataFrame()

def get_ai_decision_data(db_conn, trader_id):
    """Fetches AI decision data from the database."""
    print(f"\nFetching AI decisions for trader_id: {trader_id}")
    try:
        # Filter for decisions within the analysis period
        start_time = datetime.now() - timedelta(hours=ANALYSIS_PERIOD_HOURS)
        df = pd.read_sql_query(f"SELECT * FROM ai_decisions WHERE trader_id = '{trader_id}' AND timestamp >= '{start_time.isoformat()}' ORDER BY timestamp ASC", db_conn)
        df['timestamp'] = pd.to_datetime(df['timestamp'])
        return df
    except pd.io.sql.DatabaseError as e:
        print(f"Error fetching AI decisions: {e}")
        print("It's possible the 'ai_decisions' table doesn't exist or is empty.")
        return pd.DataFrame()

def analyze_trades_and_decisions(trades_df, decisions_df):
    """
    Analyzes trades and AI decisions to link them and evaluate SL/TP.
    This is a simplified approach and might need more sophistication for complex scenarios.
    """
    if trades_df.empty or decisions_df.empty:
        print("Not enough data to perform detailed analysis.")
        return []

    analysis_results = []
    open_positions = {} # Track open positions: {symbol_side: {'entry_trade': trade_row, 'ai_decision': decision_row}}

    # Sort both by timestamp to process chronologically
    trades_df = trades_df.sort_values(by='timestamp')
    decisions_df = decisions_df.sort_values(by='timestamp')

    # Iterate through decisions and trades to link them
    for _, decision_row in decisions_df.iterrows():
        try:
            decisions_list = json.loads(decision_row['decision_json'])
            if not isinstance(decisions_list, list):
                decisions_list = [decisions_list] # Handle single object case
        except (json.JSONDecodeError, TypeError):
            decisions_list = []

        for decision in decisions_list:
            symbol = decision.get('symbol')
            action = decision.get('action')
            
            if not symbol or not action:
                continue

            position_key = f"{symbol}_{action.split('_')[1]}" if action.startswith('open_') or action.startswith('close_') else None

            if action.startswith('open_'):
                # Find the corresponding trade(s) that happened shortly after this decision
                # This is a heuristic: look for trades within a short window after the decision
                trade_window_start = decision_row['timestamp']
                trade_window_end = decision_row['timestamp'] + timedelta(minutes=5) # Assume trade executes within 5 mins

                matching_trades = trades_df[
                    (trades_df['timestamp'] >= trade_window_start) &
                    (trades_df['timestamp'] <= trade_window_end) &
                    (trades_df['symbol'] == symbol)
                ]
                
                if not matching_trades.empty:
                    # For simplicity, take the first trade as the entry
                    entry_trade = matching_trades.iloc[0]
                    open_positions[position_key] = {
                        'entry_trade': entry_trade,
                        'ai_decision': decision,
                        'decision_timestamp': decision_row['timestamp'],
                        'cot_trace': decision_row['cot_trace']
                    }
                    # print(f"Opened {position_key} at {entry_trade['price']} based on decision at {decision_row['timestamp']}")
                else:
                    # print(f"Decision to {action} {symbol} at {decision_row['timestamp']} but no matching trade found.")
                    pass # Decision to open, but no trade executed (e.g., validation failed)

            elif action.startswith('close_') and position_key in open_positions:
                # Find the corresponding trade(s) that happened shortly after this decision
                trade_window_start = decision_row['timestamp']
                trade_window_end = decision_row['timestamp'] + timedelta(minutes=5)

                matching_trades = trades_df[
                    (trades_df['timestamp'] >= trade_window_start) &
                    (trades_df['timestamp'] <= trade_window_end) &
                    (trades_df['symbol'] == symbol)
                ]

                if not matching_trades.empty:
                    exit_trade = matching_trades.iloc[0] # For simplicity, take the first trade as the exit
                    entry_info = open_positions.pop(position_key) # Remove from open positions

                    entry_price = entry_info['entry_trade']['price']
                    exit_price = exit_trade['price']
                    
                    # Calculate PnL (simplified, not accounting for quantity or partial closes perfectly)
                    pnl = 0.0
                    if 'long' in position_key:
                        pnl = (exit_price - entry_price) * entry_info['entry_trade']['quantity']
                    elif 'short' in position_key:
                        pnl = (entry_price - exit_price) * entry_info['entry_trade']['quantity']
                    
                    # Deduct commissions (simplified, assuming commission asset is the same as trade asset)
                    total_commission = entry_info['entry_trade']['commission'] + exit_trade['commission']
                    pnl -= total_commission

                    analysis_results.append({
                        'symbol': symbol,
                        'entry_timestamp': entry_info['entry_trade']['timestamp'],
                        'entry_price': entry_price,
                        'exit_timestamp': exit_trade['timestamp'],
                        'exit_price': exit_price,
                        'ai_sl': entry_info['ai_decision'].get('stop_loss'),
                        'ai_tp': entry_info['ai_decision'].get('take_profit'),
                        'realized_pnl': pnl,
                        'ai_cot': entry_info['cot_trace'],
                        'ai_decision_json': entry_info['ai_decision']
                    })
                # else:
                    # print(f"Decision to {action} {symbol} at {decision_row['timestamp']} but no matching exit trade found.")
    
    # Handle any remaining open positions (they might not have closed within the analysis period)
    for position_key, entry_info in open_positions.items():
        analysis_results.append({
            'symbol': entry_info['entry_trade']['symbol'],
            'entry_timestamp': entry_info['entry_trade']['timestamp'],
            'entry_price': entry_info['entry_trade']['price'],
            'exit_timestamp': 'N/A (Still Open)',
            'exit_price': 'N/A',
            'ai_sl': entry_info['ai_decision'].get('stop_loss'),
            'ai_tp': entry_info['ai_decision'].get('take_profit'),
            'realized_pnl': 'N/A (Still Open)',
            'ai_cot': entry_info['cot_trace'],
            'ai_decision_json': entry_info['ai_decision']
        })

    return analysis_results


def main():
    """Main function to connect to the DB and run the analysis."""
    try:
        conn = sqlite3.connect(DB_FILE)
        print(f"Successfully connected to database: {DB_FILE}")

        decisions_df = get_ai_decision_data(conn, TRADER_ID)
        
        conn.close()

        print(f"\n--- Searching for 'move_sl_to_breakeven' decisions for trader '{TRADER_ID}' ---")
        
        found_count = 0
        for _, row in decisions_df.iterrows():
            try:
                decisions_list = json.loads(row['decision_json'])
                if not isinstance(decisions_list, list):
                    decisions_list = [decisions_list]

                for decision in decisions_list:
                    if decision.get('action') == 'move_sl_to_breakeven':
                        found_count += 1
                        print("\n" + "="*80)
                        print(f"Found 'move_sl_to_breakeven' decision!")
                        print(f"Decision ID: {row['decision_id']}, Timestamp: {row['timestamp']}")
                        print(json.dumps(decision, indent=2, ensure_ascii=False))
                        print("="*80)

            except (json.JSONDecodeError, TypeError):
                continue
        
        if found_count == 0:
            print("\nNo 'move_sl_to_breakeven' decisions were found in the database for this trader.")
        else:
            print(f"\nFound a total of {found_count} 'move_sl_to_breakeven' decision(s).")

        print("\n\nAnalysis script finished.")

    except sqlite3.Error as e:
        print(f"Database error: {e}")
        print(f"Please ensure the database file '{DB_FILE}' exists in the same directory and is a valid SQLite database.")
    except Exception as e:
        print(f"An unexpected error occurred: {e}")


if __name__ == "__main__":
    main()