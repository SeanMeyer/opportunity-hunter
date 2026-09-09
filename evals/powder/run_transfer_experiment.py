"""Explicit paid transfer check: unchanged short-with-hints prompt, Flash only.
Two new cases, two independent samples each. No search, DB writes or notifications.
"""
import concurrent.futures
import json
import run_cascades_experiment as experiment

def main():
    experiment.OUT = experiment.BASE / 'runs/20260909-transfer'
    experiment.OUT.mkdir(parents=True, exist_ok=True)
    reopening = json.loads((experiment.BASE / 'cases/cascades-confirmed-reopening.json').read_text(encoding='utf8'))
    compact = experiment.evidence()['compact']
    compact['scenario_assumptions'] = reopening['input']['scenario_assumptions']
    local = json.loads((experiment.BASE / 'cases/windy-local-i70.json').read_text(encoding='utf8'))
    inputs = {'reopening': compact, 'i70': {'as_of':local['as_of'], 'subscriber':local['profile'], 'evidence':local['input']}}
    env = dict(line.split('=',1) for line in (experiment.BASE.parents[1]/'.env').read_text().splitlines()
               if '=' in line and not line.startswith('#'))
    jobs = [('gemini-3.8-flash','hints',case,repeat) for case in inputs for repeat in [1,2]]
    with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
        for result in pool.map(lambda job:experiment.run(job,inputs,env['GOOGLE_API_KEY'].strip()),jobs):
            print(result,flush=True)

if __name__ == '__main__':
    main()
