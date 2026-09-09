"""Two diagnostic Pro calls using the same calibrated prompt and extraction."""
import concurrent.futures
import json
import run_session_validation as e
from run_calibrated_validation import CALIBRATION

def main():
 e.ADVISOR+=CALIBRATION
 e.MODEL='gemini-3.1-pro-preview'
 env=dict(line.split('=',1) for line in (e.BASE.parents[1]/'.env').read_text().splitlines() if '=' in line and not line.startswith('#'))
 schema=json.loads((e.BASE/'session-schema.json').read_text())
 with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
  for result in pool.map(lambda c:e.run((c,'calibrated-pro-1'),env['GOOGLE_API_KEY'].strip(),schema),['japan-alternative','steamboat-reopening']):print(result,flush=True)

if __name__=='__main__':main()
