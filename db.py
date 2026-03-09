import sqlite3
from datetime import datetime
from typing import List, Dict, Any

DB_PATH = 'factcheck.db'

def init_db():
    conn = sqlite3.connect(DB_PATH)
    c = conn.cursor()
    c.execute('''
        CREATE TABLE IF NOT EXISTS fact_checks (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            query TEXT NOT NULL,
            summary TEXT NOT NULL,
            sources TEXT NOT NULL,
            timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
        )
    ''')
    conn.commit()
    conn.close()

def add_fact_check(query: str, summary: str, sources: str) -> int:
    conn = sqlite3.connect(DB_PATH)
    c = conn.cursor()
    c.execute(
        'INSERT INTO fact_checks (query, summary, sources, timestamp) VALUES (?, ?, ?, ?)',
        (query, summary, sources, datetime.utcnow())
    )
    conn.commit()
    last_id = c.lastrowid
    conn.close()
    return last_id

def get_fact_checks() -> List[Dict[str, Any]]:
    conn = sqlite3.connect(DB_PATH)
    conn.row_factory = sqlite3.Row
    c = conn.cursor()
    c.execute('SELECT * FROM fact_checks ORDER BY timestamp DESC')
    rows = c.fetchall()
    conn.close()
    return [dict(row) for row in rows]
