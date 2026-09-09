"""One directed refinement; same fixtures, two repeats on difficult cases."""
import concurrent.futures
import json
import run_session_validation as e

CALIBRATION = ''' Keep advice decisive but calibrate its claims: weather and geography suggest opportunities, not confirmed lift operations, preserved depth or exact surface conditions. If the plan depends on an unconfirmed fact, put that condition in the recommendation itself. Compare your preferred plan with its strongest practical alternative before choosing. Support decision-changing external claims with sources; if not verified, say so. Do not turn a value reference into a trip quote.'''

def main():
 e.ADVISOR+=CALIBRATION
 env=dict(line.split('=',1) for line in (e.BASE.parents[1]/'.env').read_text().splitlines() if '=' in line and not line.startswith('#'))
 schema=json.loads((e.BASE/'session-schema.json').read_text())
 jobs=[(c,'calibrated-1') for c in e.CASES]+[(c,'calibrated-2') for c in ['japan-alternative','steamboat-reopening']]
 with concurrent.futures.ThreadPoolExecutor(max_workers=3) as pool:
  for result in pool.map(lambda j:e.run(j,env['GOOGLE_API_KEY'].strip(),schema),jobs):print(result,flush=True)

if __name__=='__main__':main()
