"""Explicit paid search-enabled historical-terrain probe; no app side effects."""
import json
import time
import sys
import urllib.request
import run_cascades_experiment as experiment

def main():
    case=json.loads((experiment.BASE/'cases/windy-local-i70.json').read_text(encoding='utf8'))
    evidence={'as_of':case['as_of'],'subscriber':case['profile'],'evidence':case['input']}
    prompt=experiment.HINTS.replace(
        'This is an offline historical case: use only the supplied evidence, not current search or present-day knowledge.',
        'This is a historical storm. Keep its archived date and forecast fixed. You may use general knowledge and Google Search to investigate terrain, wind exposure and potential alternatives. Current operating reports cannot prove historical access. Explain which useful claims come from sources and which remain hypotheses; if direction is missing, do not invent it.')
    prompt+='\n\n'+experiment.encode(evidence)
    if '--require-search' in sys.argv:
        prompt+='\n\nUse Google Search now to investigate whether terrain or wind exposure at any of the listed resorts suggests a worthwhile alternative. Report relevant sources and what they do or do not establish. General terrain information may inform hypotheses; keep historical operations and storm wind direction unknown where missing.'
    out=experiment.BASE/('runs/20260909-required-search-probe.json' if '--require-search' in sys.argv else 'runs/20260909-search-probe.json')
    if out.exists():
        print('Existing result preserved');return
    env=dict(line.split('=',1) for line in (experiment.BASE.parents[1]/'.env').read_text().splitlines() if '=' in line and not line.startswith('#'))
    payload={'contents':[{'role':'user','parts':[{'text':prompt}]}],
        'tools':[{'googleSearch':{}}],
        'generationConfig':{'thinkingConfig':{'thinkingLevel':'MEDIUM'},'maxOutputTokens':12000}}
    req=urllib.request.Request('https://generativelanguage.googleapis.com/v1beta/models/gemini-3.8-flash:generateContent',data=json.dumps(payload).encode(),headers={'Content-Type':'application/json','x-goog-api-key':env['GOOGLE_API_KEY'].strip()})
    started=time.monotonic()
    try:
        with urllib.request.urlopen(req,timeout=180) as response:data=json.load(response)
    except Exception as error:
        print(type(error).__name__);return
    candidate=data['candidates'][0]
    text='\n'.join(p.get('text','') for p in candidate.get('content',{}).get('parts',[]) if not p.get('thought'))
    record={'model':'gemini-3.8-flash','prompt':prompt,'assessment':text,'finish_reason':candidate.get('finishReason'),'grounding':candidate.get('groundingMetadata',{}),'usage':data.get('usageMetadata',{}),'elapsed_seconds':time.monotonic()-started,'method':'Exploratory search-enabled probe, historical forecast fixed; no live historical-operations validation, no schema extraction.'}
    with out.open('x',encoding='utf8') as f:f.write(experiment.encode(record)+'\n')
    print(record['finish_reason'], 'grounding sources:',len(record['grounding'].get('groundingChunks',[])))
    print(text)

if __name__=='__main__':main()
