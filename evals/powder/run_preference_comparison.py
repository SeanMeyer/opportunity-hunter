"""Controlled preference sensitivity test of v5. Paid; no DB or notifications.
Fictional resorts remove current operations/geography differences. Two samples per profile.
"""
import concurrent.futures
import hashlib
import json
import re
import run_session_validation as e
from run_extraction_validation import BASE_PROMPT, ADDITION

OUT=e.BASE/'runs/20260909-preferences'
PREFERENCES={
 'neutral':'No particular terrain preference.',
 'trees':'Tree skiing is my favorite. I would usually choose accessible gladed runs over exposed alpine bowls or groomed pistes. Fresh snow is a bonus, but I do not need the deepest headline total.',
 'untouched':'My main goal is untouched powder. I am willing to wait for a possible terrain opening or accept some uncertainty for a meaningful chance of untracked snow. Lift-served terrain only; this is not a request for backcountry skiing.',
 'groomers':'I love freshly groomed corduroy and predictable, smooth piste skiing. I prefer groomed runs to deep powder or tree skiing; a big snowfall is not itself a benefit to me.'
}
EVIDENCE={
 'as_of':'2027-01-05T17:00:00-07:00',
 'method':'Entirely hypothetical, fictional resorts. Use only this fixed evidence, not search or invented resort knowledge. All options are equal two-hour drives from Denver, covered by the stipulated pass with no extra ticket cost, and have confirmed open road access. One ski day is available, either Wednesday Jan 6 or Thursday Jan 7. Lodging is unnecessary. All skiing is lift-served in-bounds. No measured crowds or preserved powder depths. Opening snow spans previous 16:00 to 09:00; during skiing spans 09:00–16:00 local.',
 'options':[
  {'resort':'Cedar Ridge','Wednesday':'7 inches opening snow, 1 inch during skiing. Broad lift-served glades confirmed open; below freezing, gusts 15 mph. Some pistes groomed before the last snowfall; they have a soft fresh covering rather than clean corduroy.','Thursday':'1 inch opening, none during skiing. Same terrain open; Wednesday snow likely partly tracked, no untouched-depth measurement.'},
  {'resort':'Summit Peak','Wednesday':'16 inches opening snow, 4 inches during skiing. Upper bowls closed for storm recovery; only lower groomers open. Gusts 45 mph. No tree skiing supplied.','Thursday':'2 inches opening, none during skiing; gusts ease to 15 mph, below freezing. Upper bowls might reopen Thursday after patrol assessment, but no confirmed opening time. Prior snow could remain untracked in closed terrain; wind redistribution and patrol work make retained depth unknown. If bowls stay closed, only lower groomers available.'},
  {'resort':'Valley Glide','Wednesday':'2 inches of snow before grooming finished at 08:30. Wide groomed pistes and lifts confirmed open at 09:00, no additional snow during skiing. Below freezing, gusts 10 mph.','Thursday':'No new snow. Overnight grooming and normal lift operations planned, not yet observed. No gladed or alpine powder terrain supplied.'}
 ]
}

def render(template,preference):
 values={'StormWindow':'Regional snowfall opportunity, three resort/day choices.','RegionName':'Fictional test region','WeatherData':json.dumps(EVIDENCE,ensure_ascii=False,indent=2),'ModelConsensus':'No additional models; use scenario values.','ForecastDiscussion':'No discussion. Fictional case: no external research.','Resorts':'See the three named fictional options above.','UserProfile':'Home: Denver\nPass: all three fictional resorts covered\nSkill: expert\nPTO: 15 days\nSchedule: one ski day, Wednesday or Thursday\nPreferences: '+preference,'EvaluationHistory':'No prior evaluation.','SubscriberFeedback':'No feedback.','PromptVersion':'v5.0.0'}
 for k,v in values.items():template=template.replace('{{.'+k+'}}',v)
 assert '{{.' not in template
 return template

def main():
 OUT.mkdir(parents=True,exist_ok=True)
 path=OUT/'template.txt'
 if path.exists():template=path.read_text(encoding='utf8')
 else:
  template=re.search(r'const stormEvalPromptTemplate = `([\s\S]*?)`',(e.BASE.parents[1]/'hunts/powder/prompt.go').read_text()).group(1)
  path.write_text(template,encoding='utf8')
 fixture=OUT/'fixture.json'
 if not fixture.exists():fixture.write_text(json.dumps({'evidence':EVIDENCE,'preferences':PREFERENCES},indent=2),encoding='utf8')
 env=dict(line.split('=',1) for line in (e.BASE.parents[1]/'.env').read_text().splitlines() if '=' in line and not line.startswith('#'))
 schema=json.loads((e.BASE/'session-schema.json').read_text())
 def run(job):
  profile,repeat=job;dest=OUT/f'{profile}-{repeat}.json'
  if dest.exists():return dest.name+': preserved'
  prompt=render(template,PREFERENCES[profile])
  record={'profile':profile,'repeat':repeat,'model':e.MODEL,'prompt':prompt,'prompt_sha256':hashlib.sha256(prompt.encode()).hexdigest(),'evidence_sha256':hashlib.sha256(json.dumps(EVIDENCE,sort_keys=True).encode()).hexdigest(),'method':'Fixed fictional choices; only Preferences changes. Production v5 template with matched rendered context, actual powder schema. Google Search tool available as in production but scenario explicitly disallows external research; grounding activity recorded. Not an end-to-end ingestion test.'}
  try:
   record['research']=e.request(env['GOOGLE_API_KEY'].strip(),prompt)
   if record['research']['finish_reason']!='STOP':raise ValueError('Incomplete research')
   extraction=BASE_PROMPT+'\n\n## Original context\n'+prompt+'\n\n## Analysis\n\n'+record['research']['text']+'\n\n## Extraction fidelity\n'+ADDITION
   record['extraction_prompt']=extraction
   record['extraction']=e.request(env['GOOGLE_API_KEY'].strip(),extraction,schema)
   if record['extraction']['finish_reason']!='STOP':raise ValueError('Incomplete extraction')
   record['structured']=json.loads(record['extraction']['text'])
  except Exception as error:record['error']=type(error).__name__
  with dest.open('x',encoding='utf8') as f:json.dump(record,f,ensure_ascii=False,indent=2)
  return dest.name+': '+record.get('error',str(record.get('structured',{}).get('tier')))
 with concurrent.futures.ThreadPoolExecutor(max_workers=3) as pool:
  for result in pool.map(run,[(p,r) for p in PREFERENCES for r in [1,2]]):print(result,flush=True)

if __name__=='__main__':main()
