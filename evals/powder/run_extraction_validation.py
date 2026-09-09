"""Extraction-only counterexamples with human-authored research; no search."""
import concurrent.futures
import json
import run_session_validation as e

CASES = {
 'conditional': 'WATCH. Niseko Thursday could be worthwhile if upper lifts reopen, but that is unconfirmed. Wednesday Rusutsu is an alternative only if BOTH road access and resort lift operations confirm before driving. Lower forecast gusts suggest a possibility, not confirmed operations. No price researched. Check official operations tomorrow; that is a future task, not a cited source.',
 'budget': 'RECOMMENDED, conditional on road access and acceptable booking costs. An approximately $800 trip would be worthwhile for this skier, but no actual travel estimate or quote is available. Lodging, flight and ticket costs are Unknown. Do not book until these are checked. No sources were retrieved.',
 'closure': 'RECOMMENDED: target Friday at Steamboat if the announced reopening proceeds with usable terrain. 14 inches during closure plus 6 inches before opening totals 20 inches of snowfall, not a measurement or promise of retained powder. Friday has a plausible weekday advantage; Saturday adds 12 inches and could be better if terrain releases slip. No crowd observation or terrain release schedule is confirmed. Lodging is an unverified $180 per-night estimate; total trip cost unknown.'
}
BASE_PROMPT = '''Extract the analysis into the required JSON schema. Preserve the analyst's verdict,
reasoning summary, scores, uncertainty, and caveats; do not re-evaluate or strengthen the recommendation.
The original context is included to resolve references and IDs, not to invent missing conclusions.
For unavailable string details use "Unknown"; for inapplicable details use "Not applicable".
Use empty arrays when no entries are supported. Never manufacture prices or sources.'''
ADDITION = '''Only list sources actually cited or retrieved, never suggested future checks. Keep illustrative budgets distinct from trip cost estimates. Preserve decisive conditions in both recommendation and summary; a possible reopening must not become a confirmed reopening in any field.
Retrieved source URLs:
[]'''

def main():
 env=dict(line.split('=',1) for line in (e.BASE.parents[1]/'.env').read_text().splitlines() if '=' in line and not line.startswith('#'))
 schema=json.loads((e.BASE/'session-schema.json').read_text())
 def run(job):
  case,variant=job
  dest=e.OUT/f'extraction-{case}-{variant}.json'
  if dest.exists():return f'{case}-{variant}: preserved'
  prompt=BASE_PROMPT+'\n\n## Original context\nNo additional evidence.\n\n## Analysis\n'+CASES[case]
  if variant=='updated':prompt+='\n'+ADDITION
  result=e.request(env['GOOGLE_API_KEY'].strip(),prompt,schema)
  if result['finish_reason']!='STOP':raise ValueError('Incomplete extraction')
  with dest.open('x',encoding='utf8') as f:json.dump({'method':'Human-authored counterexample, extraction-only. Not a research quality test.','case':case,'variant':variant,'model':e.MODEL,'prompt':prompt,'extraction':result,'structured':json.loads(result['text'])},f,ensure_ascii=False,indent=2)
  return f'{case}-{variant}: '+result['finish_reason']
 with concurrent.futures.ThreadPoolExecutor(max_workers=3) as pool:
  for result in pool.map(run,[(c,v) for c in CASES for v in ['original','updated']]):print(result,flush=True)

if __name__=='__main__':main()
