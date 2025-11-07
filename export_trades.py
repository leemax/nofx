import sqlite3
import json

db_path = '/Users/lichungang/nofx/nofx.db'
output_file = '/Users/lichungang/nofx/latest_30_trades_export.txt' # New output file name

conn = None
try:
    conn = sqlite3.connect(db_path)
    cursor = conn.cursor()

    # Query the last 30 trades, ordered by timestamp descending
    query = "SELECT * FROM trades ORDER BY timestamp DESC LIMIT 30"
    cursor.execute(query)
    rows = cursor.fetchall()

    column_names = [description[0] for description in cursor.description]

    with open(output_file, 'w', encoding='utf-8') as f:
        f.write("--- Export of latest 30 trades from {} ---\n\n".format(db_path))
        if not rows:
            f.write("No trades found in the database.\n")
        for row in rows:
            row_dict = dict(zip(column_names, row))
            f.write("--- Trade ID: {} ---\n".format(row_dict['trade_id']))
            for col_name, col_value in row_dict.items():
                f.write("{}: {}\n".format(col_name, col_value))
            f.write("\n" + "="*80 + "\n\n")

    print("Data successfully exported to {}".format(output_file))
    if not rows:
        print("Warning: No trades were found in the database.")

except sqlite3.Error as e:
    print("SQLite error: {}".format(e))
except Exception as e:
    print("An unexpected error occurred: {}".format(e))
finally:
    if conn:
        conn.close()
