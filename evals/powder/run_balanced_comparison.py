"""Directed refinement of the matched comparison's strongest overall candidate."""
import concurrent.futures
import run_tradeoff_comparison as r

def main():
 template,shared,schema=r.templates();cases=r.fixtures()
 env=dict(line.split('=',1) for line in (r.e.BASE.parents[1]/'.env').read_text().splitlines() if '=' in line and not line.startswith('#'))
 jobs=[(c,'balanced-1') for c in cases]+[(c,'balanced-2') for c in ['whitepass-delay','steamboat-reopening']]
 with concurrent.futures.ThreadPoolExecutor(max_workers=3) as pool:
  for result in pool.map(lambda j:r.run(j,env['GOOGLE_API_KEY'].strip(),template,shared,schema,cases),jobs):print(result,flush=True)

if __name__=='__main__':main()
