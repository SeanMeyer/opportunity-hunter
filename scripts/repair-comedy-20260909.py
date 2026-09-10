"""Audited one-time repair. Stop the daemon and back up SQLite before invoking.

Usage: python3 repair-comedy-20260909.py /path/to/opportunity-hunter.db
Requires the v0.3.4 schema. Preserves all opportunity, pick and feedback rows.
"""
import datetime
import json
import sqlite3
import sys


def repair(path):
    db = sqlite3.connect(path)
    db.row_factory = sqlite3.Row
    db.execute('PRAGMA foreign_keys=ON')
    try:
        db.execute('BEGIN IMMEDIATE')
        counts = [db.execute('SELECT count(*) FROM ' + t).fetchone()[0]
                  for t in ('opportunities', 'picks', 'feedback')]
        rows = {r['id']: r for r in db.execute(
            'SELECT * FROM opportunities WHERE id IN (40,344,311,312)')}
        assert len(rows) == 4, 'Expected audited records are missing'
        old, new, hasan, alias = [rows[i] for i in (40, 344, 311, 312)]
        assert all(r['hunt_name'] == 'comedy' for r in rows.values())
        assert old['title'] == new['title'] == 'Sam Jay'
        assert old['source'] == new['source'] == 'ticketmaster'
        assert old['source_id'] == new['source_id'] == 'rZ7HnEZ1AfqZ_K'
        assert old['ticket_url'] == new['ticket_url'] == 'https://www.ticketweb.com/event/sam-jay-denver-improv-tickets/14740653'
        assert old['superseded_by'] is None and new['superseded_by'] in (None, 40)
        assert old['start_time'].startswith(('2026-11-20', '2027-01-22'))
        dates = json.loads(new['show_dates'])
        assert len(dates) == 2 and dates[0].startswith('2027-01-22') and dates[1].startswith('2027-01-23')
        assert hasan['title'] == 'HASAN HATES RONNY | RONNY HATES HASAN'
        assert alias['title'] == 'Hasan Minhaj w/ Ronny Chieng'
        assert hasan['source_id'] == 'G5vzZ_G9Uho9f' and alias['source_id'] == 'Z7r9jZ1A7PaJF'
        assert hasan['superseded_by'] is None and alias['superseded_by'] in (None, 311)
        assert datetime.datetime.fromisoformat(hasan['start_time'].replace('Z', '+00:00')) == datetime.datetime.fromisoformat(alias['start_time'].replace('Z', '+00:00'))
        assert hasan['start_time'].startswith('2026-09-24')
        # Current provider schedule replaces the verified obsolete November dates.
        db.execute('UPDATE opportunities SET start_time=?,end_time=?,show_dates=?,attributes=?,raw_data=?,price_min=?,price_max=? WHERE id=40',
                   tuple(new[k] for k in ('start_time', 'end_time', 'show_dates', 'attributes', 'raw_data', 'price_min', 'price_max')))
        for canonical, duplicate in ((40, 344), (311, 312)):
            db.execute('UPDATE opportunities SET superseded_by=? WHERE id=? OR superseded_by=?',
                       (canonical, duplicate, duplicate))
        assert counts == [db.execute('SELECT count(*) FROM ' + t).fetchone()[0]
                          for t in ('opportunities', 'picks', 'feedback')]
        assert not db.execute('PRAGMA foreign_key_check').fetchall()
        db.commit()
        print('Audited comedy repairs applied; all history rows retained')
    finally:
        db.close()


if __name__ == '__main__':
    repair(sys.argv[1])
