import sqlite3
import json

db_path = '/Users/lichungang/nofx/nofx.db'
output_file = '/Users/lichungang/nofx/ai_decisions_export.txt'
start_row = 486
end_row = 621

conn = None
try:
    conn = sqlite3.connect(db_path)
    cursor = conn.cursor()

    query = f"SELECT * FROM ai_decisions WHERE decision_id BETWEEN {start_row} AND {end_row} ORDER BY decision_id ASC"
    cursor.execute(query)
    rows = cursor.fetchall()

    column_names = [description[0] for description in cursor.description]

    with open(output_file, 'w', encoding='utf-8') as f:
        f.write(f"--- Export of ai_decisions table from {db_path} (Rows {start_row} to {end_row}) ---\n\n")
        for row in rows:
            row_dict = dict(zip(column_names, row))
            f.write(f"--- Record ID: {row_dict['decision_id']} ---\n")
            for col_name, col_value in row_dict.items():
                if col_name == 'decision_json' and col_value:
                    try:
                        # Pretty print JSON for readability
                        f.write(f"{col_name}:\n{json.dumps(json.loads(col_value), indent=2, ensure_ascii=False)}\n")
                    except json.JSONDecodeError:
                        f.write(f"{col_name}: {col_value}\n")
                elif col_name == 'input_prompt' or col_name == 'cot_trace':
                    f.write(f"{col_name}:\n{col_value}\n")
                else:
                    f.write(f"{col_name}: {col_value}\n")
            f.write("\n" + "="*80 + "\n\n")

    print(f"Data successfully exported to {output_file}")

except sqlite3.Error as e:
    print(f"SQLite error: {e}")
except Exception as e:
    print(f"An unexpected error occurred: {e}")
finally:
    if conn:
        conn.close()
