#!/usr/bin/env python3
"""Check Markdown inventory, current local links, fences and generated SVG drift.

No network, credentials, services or dependencies. Historical/vendored link
failures are reported separately; their original records are not rewritten.
"""
from pathlib import Path
import argparse
import json
import os
import re
import subprocess
import sys
import urllib.parse
import xml.etree.ElementTree as ET

ROOT = Path(__file__).resolve().parents[1]
MAP = ROOT / 'docs/DOCUMENTATION_MAP.md'
HISTORICAL_ROOT = {
    'docs/ROLES_30_60_90.md', 'docs/VETO_LIST_AUDIT.md',
    'docs/DOCS_SANITIZATION_AUDIT.md', 'docs/SEIDR_INTEGRATION.md',
    'docs/grovernance_integration.md', 'docs/medqcnn_integration.md',
    'docs/nexus_integration.md',
}

def inventory():
    r = subprocess.run(['git','ls-files','-co','--exclude-standard','--','*.md'],
                       cwd=ROOT, text=True, capture_output=True, check=True)
    return sorted({p for p in r.stdout.splitlines() if (ROOT/p).is_file()})

def classification(path):
    if path.startswith('.claude/'): return 'Vendored/local workflow guidance'
    if path.startswith('docs/adr/'): return 'Decision record / governance'
    if path in HISTORICAL_ROOT or path.startswith(('docs/archive/','docs/audits/','docs/research/')):
        return 'Historical observation / research'
    if path.startswith('docs/validation/'): return 'Dated executed evidence'
    if path.startswith(('docs/design/','docs/releases/')) or path=='docs/assets/keepsave-banner-prompt.md':
        return 'Dated plan / implementation / publication'
    if path in ('CLAUDE.md','docs/ADLC.md','docs/ROLES.md'):
        return 'Governing development rules'
    if path.startswith(('tests/','integrations/','frontend/','backend/')):
        return 'Source / client / fixture guide'
    return 'Current product / operator guide'

def historical(path):
    return classification(path) in {
        'Vendored/local workflow guidance','Decision record / governance',
        'Historical observation / research','Dated executed evidence',
        'Dated plan / implementation / publication',
    }

def catalog(paths):
    lines=['# KeepSave documentation map','','Reviewed 4 October 2026. This inventory covers **every repository Markdown',
           'document**, including hidden workflow guidance and client/test guides. A',
           'classification describes scope; it does not supply acceptance or signatures.',
           'Current entry points are in [the hub](README.md) and [status](STATUS.md).',
           'Historical bodies and dated execution evidence remain distinct from current',
           'source explanations. Retained client examples may still need repair/qualification.',
           '',f'**{len(paths)} documents.** Regenerate with `python3 scripts/check_docs.py --write-map`.',
           'Validate with `python3 scripts/check_docs.py`. The checker reports unresolved',
           'historical/vendored links separately; it does not imply those old examples work.',
           '', '| Document | Scope |', '|---|---|']
    for p in paths:
        target=os.path.relpath(ROOT/p,MAP.parent).replace(os.sep,'/')
        lines.append(f'| [{p}]({target}) | {classification(p)} |')
    return '\n'.join(lines)+'\n'

def headings(source):
    ids={}; used={}
    for line in source.splitlines():
        m=re.match(r'^#{1,6}\s+(.+?)\s*#*$',line)
        if not m: continue
        h=re.sub(r'<[^>]+>','',m[1]).lower().strip()
        h=re.sub(r'[^\w\- ]','',h).replace(' ','-')
        n=used.get(h,0);used[h]=n+1;ids[h if n==0 else f'{h}-{n}']=True
    return ids

def main():
    a=argparse.ArgumentParser(description=__doc__);a.add_argument('--write-map',action='store_true');a.add_argument('--json',action='store_true');args=a.parse_args()
    paths=inventory()
    if 'docs/DOCUMENTATION_MAP.md' not in paths: paths.append('docs/DOCUMENTATION_MAP.md');paths.sort()
    expected=catalog(paths)
    if args.write_map: MAP.write_text(expected)
    errors=[]; warnings=[]; links=0
    if not MAP.exists() or MAP.read_text()!=expected: errors.append({'file':'docs/DOCUMENTATION_MAP.md','issue':'inventory drift; run --write-map'})
    for name in paths:
        p=ROOT/name;source=p.read_text();unfenced=[];fence=None
        for i,line in enumerate(source.splitlines(),1):
            m=re.match(r'^\s*(`{3,}|~{3,})',line)
            if m:
                if fence is None: fence=(m[1][0],len(m[1]),i)
                elif m[1][0]==fence[0] and len(m[1])>=fence[1]: fence=None
                continue
            if fence is None: unfenced.append((i,line))
        if fence: errors.append({'file':name,'line':fence[2],'issue':'unclosed code fence'})
        for i,line in unfenced:
            for m in re.finditer(r'!?\[[^\]\n]*\]\((<[^>]+>|[^)\n]+)\)',line):
                value=m[1].strip();value=value[1:-1] if value.startswith('<') else value.split(' "',1)[0]
                url=urllib.parse.urlsplit(value)
                if url.scheme or value.startswith('//'): continue
                target=urllib.parse.unquote(url.path)
                if not target and not url.fragment: continue
                links+=1
                q=Path(target) if target.startswith('/') else p.parent/target if target else p
                issue=None
                if not q.exists(): issue='missing local target: '+value
                elif url.fragment and q.is_file() and q.suffix.lower()=='.md' and urllib.parse.unquote(url.fragment) not in headings(q.read_text()):
                    issue='missing Markdown heading: '+value
                if issue:
                    item={'file':name,'line':i,'issue':issue}
                    (warnings if historical(name) or name=='docs/DOCUMENTATION_MAP.md' else errors).append(item)
    svgs=list((ROOT/'docs/diagrams').glob('*.svg'))+[ROOT/'docs/assets/keepsave-header.svg']
    for p in svgs:
        try:
            tree=ET.parse(p);r=tree.getroot();ns='{http://www.w3.org/2000/svg}'
            if r.tag!=ns+'svg' or r.find(ns+'title') is None: raise ValueError('SVG root/title missing')
            vb=[float(x) for x in r.attrib['viewBox'].split()]
            if len(vb)!=4 or vb[2]<=0 or vb[3]<=0: raise ValueError('invalid viewBox')
            ids=[e.attrib['id'] for e in r.iter() if 'id' in e.attrib]
            if len(ids)!=len(set(ids)): raise ValueError('duplicate SVG IDs')
        except (ET.ParseError,KeyError,ValueError) as e: errors.append({'file':str(p.relative_to(ROOT)),'issue':str(e)})
    gen=subprocess.run([sys.executable,str(ROOT/'scripts/gen_diagrams.py'),'--check'],cwd=ROOT,capture_output=True,text=True)
    if gen.returncode: errors.append({'file':'scripts/gen_diagrams.py','issue':'generated diagram drift: '+gen.stderr.strip()})
    result={'status':'pass' if not errors else 'fail','documents':len(paths),'local_links_checked':links,'svg_files':len(svgs),'errors':errors,'historical_link_warnings':warnings,'generated_diagrams':'pass' if not gen.returncode else 'fail'}
    print(json.dumps(result,indent=2) if args.json else f"{result['status']}: {len(paths)} Markdown, {links} local links, {len(svgs)} SVG, {len(errors)} errors, {len(warnings)} historical warnings")
    if not args.json:
        for e in errors: print(e)
    return bool(errors)

if __name__=='__main__': sys.exit(main())
