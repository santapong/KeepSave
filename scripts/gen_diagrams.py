#!/usr/bin/env python3
"""Render the current KeepSave candidate as deterministic, branded static SVG.

The fixed dark canvas is portable across Markdown renderers. Field Twist comes
from the accepted application SVG; labels describe source, never release/UAT.
Run from any directory. No network, packages, service startup or secrets needed.
"""
from pathlib import Path
import html
import re
import textwrap
import sys

ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / 'docs/diagrams'
BG, PANEL, INK, MUTED = '#0b0a12', '#17141f', '#eeecf3', '#b4aec3'
VIOLET, MINT, BORDER = '#a78bfa', '#4fe3b8', '#685c80'
FONT = 'Geist, Inter, ui-sans-serif, system-ui, sans-serif'
MARK = (ROOT / 'frontend/public/keepsave.svg').read_text()
MARK = re.sub(r'^.*?<svg[^>]*>|</svg>\s*$', '', MARK, flags=re.S)
MARK = re.sub(r'<(?:title|desc)>.*?</(?:title|desc)>', '', MARK, flags=re.S)

def esc(value):
    return html.escape(str(value), quote=True)

def text(x, y, value, size=16, color=INK, weight=400, anchor='start'):
    return f'<text x="{x}" y="{y}" fill="{color}" font-family="{FONT}" font-size="{size}" font-weight="{weight}" text-anchor="{anchor}">{esc(value)}</text>\n'

class Diagram:
    def __init__(self, name, title, note, height=740):
        self.name, self.height = name, height
        self.nodes = {}
        self.parts = [f'<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="{height}" viewBox="0 0 1200 {height}" role="img" aria-labelledby="title desc">\n<title id="title">KeepSave — {esc(title)}</title>\n<desc id="desc">{esc(note)} Source candidate, reviewed against the October 2 implementation. External acceptance remains pending.</desc>\n',
                      f'<defs><marker id="arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M0 0L10 5L0 10Z" fill="{VIOLET}"/></marker></defs>\n',
                      f'<rect width="1200" height="{height}" rx="20" fill="{BG}"/>\n',
                      f'<g transform="translate(32 25) scale(.8)">{MARK}</g>\n',
                      text(103, 47, 'KeepSave', 22, INK, 650), text(103, 74, title, 17, VIOLET, 550),
                      text(40, 108, note, 16, MUTED),
                      f'<path d="M40 {height-91}H1160" stroke="{BORDER}"/>\n',
                      text(40, height-56, 'SOURCE CANDIDATE · PostgreSQL platform · New admission flags default off', 15, MINT, 500),
                      text(40, height-29, 'Real providers, harnesses, runner isolation and production acceptance remain separate gates.', 14, MUTED)]

    def node(self, key, x, y, label, detail='', width=240, height=108, kind='service'):
        colors = {'service':VIOLET, 'store':MINT, 'external':BORDER, 'security':'#d6c9ff', 'runner':'#e7a566'}
        self.nodes[key] = (x,y,width,height)
        self.parts.append(f'<rect x="{x}" y="{y}" width="{width}" height="{height}" rx="14" fill="{PANEL}" stroke="{colors[kind]}" stroke-width="1.5"/>\n')
        lines = label.split('\n')
        for i,line in enumerate(lines):
            if len(line)>34: raise ValueError(f'{key}: long title {line!r}')
            self.parts.append(text(x+width/2,y+32+i*23,line,19,INK,600,'middle'))
        details = [line for part in detail.split('\n') for line in textwrap.wrap(part, max(32,int((width-28)/7)))] if detail else []
        for i,line in enumerate(details):
            if len(line) > max(32, int((width-28)/7)): raise ValueError(f'{key}: long detail {line!r}')
            self.parts.append(text(x+width/2,y+34+len(lines)*23+i*20,line,14,MUTED,400,'middle'))
        if 34+len(lines)*23+max(0,len(details)-1)*20>height-8:
            raise ValueError(f'{key}: text exceeds box')

    def edge(self, start, end, label='', reverse=False):
        x,y,w,h = self.nodes[start]; xx,yy,ww,hh = self.nodes[end]
        if y==yy:
            pts = [(x+w,y+h/2),(xx,yy+hh/2)] if x<xx else [(x,y+h/2),(xx+ww,yy+hh/2)]
            lx,ly=(pts[0][0]+pts[-1][0])/2,y+h+25
            if label:
                self.parts.append(f'<path d="M{lx} {pts[0][1]+5}V{ly-16}" stroke="{BORDER}" stroke-dasharray="3 3" fill="none"/>\n')
        elif x==xx:
            pts = [(x+w/2,y+h),(xx+ww/2,yy)] if y<yy else [(x+w/2,y),(xx+ww/2,yy+hh)]
            lx,ly=pts[0][0]+82,(pts[0][1]+pts[-1][1])/2
        else:
            raise ValueError('Only aligned edges: author a clear rail instead of ambiguous crossing routes')
        if reverse: pts.reverse()
        self.line(pts,label,lx,ly)

    def line(self, pts, label='', lx=None, ly=None):
        self.parts.append(f'<path d="M{pts[0][0]} {pts[0][1]}'+''.join(f'L{x} {y}' for x,y in pts[1:])+f'" fill="none" stroke="{VIOLET}" stroke-width="1.8" marker-end="url(#arrow)"/>\n')
        if label:
            for i,line in enumerate(textwrap.wrap(label,18)):
                self.parts.append(text(lx,ly+i*16,line,13,MUTED,500,'middle'))

    def rail(self, y, heading, nodes, edges=True):
        self.parts.append(text(40,y-19,heading,15,MINT,550))
        keys=[]
        for i,node in enumerate(nodes):
            key,label,detail,*kind = node
            self.node(key,40+i*290,y,label,detail,kind=kind[0] if kind else 'service')
            keys.append(key)
        if edges:
            for a,b in zip(keys,keys[1:]): self.edge(a,b)
        return keys

    def save(self):
        body = '\n'.join(line.rstrip() for line in (''.join(self.parts)+'</svg>').splitlines())+'\n'
        if '--check' in sys.argv:
            if not (OUT/self.name).exists() or (OUT/self.name).read_text()!=body:
                raise ValueError(f'{self.name}: regenerate diagrams')
        else:
            OUT.mkdir(parents=True,exist_ok=True)
            (OUT/self.name).write_text(body)
        print(f'{self.name}: {len(self.nodes)} nodes')

# Context: the three admission paths share a platform; external systems are distinct.
d=Diagram('c4-1-context.svg','System context','Developer teams manage a vault and approve bounded developer-tool access.',800)
for i,(key,label,detail) in enumerate([('member','Team member','Web workspace / account'),('harness','Developer harness','Codex / Hermes candidates'),('client','Existing vault clients','CLI / SDK / widget / CI')]):
    d.node(key,40+i*390,155,label,detail,width=340,kind='external')
d.node('platform',40,342,'KeepSave platform','Identity · current policy · vault · runs · broker · durable audit',width=1120,height=100)
for i,(key,label,detail) in enumerate([('login','Google / GitHub sign-in','Separate social identities'),('github','GitHub App','Read-only tool provider'),('keys','Transit / SMTP','Key custody / gated email')]):
    d.node(key,40+i*390,540,label,detail,width=340,kind='external')
for i,key in enumerate(['member','harness','client']):
    x=210+i*390; d.line([(x,263),(x,342)],['human session','client delegation','scoped vault key'][i],x+96,304)
for i,key in enumerate(['login','github','keys']):
    x=210+i*390; d.line([(x,442),(x,540)],['verified subject','broker-held token','trusted adapters'][i],x+96,493)
d.save()

d=Diagram('c4-2-container.svg','Container and host boundaries','The control host owns vault custody; the separate runner has no database or vault access.')
d.rail(163,'CONTROL HOST · same-origin application',[('frontend','Frontend / TLS','app.keepsave.draveniq.dev'),('api','Go / Gin API','REST + stateless MCP'),('db','PostgreSQL 16','Authority / audit / jobs','store'),('worker','Trusted worker','Backups / mail / exports')],False)
d.edge('frontend','api','HTTPS');d.edge('api','db','SQL');d.edge('db','worker','leases')
d.rail(444,'EXECUTION PATH · broker remains on the control host',[('connector','Pinned connector','No IP network / no tokens','runner'),('supervisor','Runner supervisor','Podman / Unix relay','runner'),('broker','Trusted broker','Private mTLS admission','security'),('provider','GitHub API','Stored repository / commit','external')],False)
d.edge('connector','supervisor','Unix relay');d.edge('supervisor','broker','mTLS ticket');d.edge('broker','provider','read API')
d.edge('supervisor','api','claim / complete')
d.save()

d=Diagram('c4-3-component-api.svg','Application component ownership','Transport adapters share authorized services; legacy adapters remain explicit exceptions.')
d.rail(163,'REQUEST PATH',[('transport','REST / MCP / CLI','Translate requests'),('service','Authorized services','Stored authority / scopes','security'),('repository','Module repositories','Transactions / narrow ports'),('database','PostgreSQL','Local state + audit + outbox','store')])
d.rail(444,'DOMAIN OWNERS · no harness-specific policy types',[('identity','Identity / policy','Sessions / epochs / decisions','security'),('vault','Vault / promotion','Ciphertext / revisions / keys','security'),('access','Runs / broker','Client-bound grant / custody','security'),('automation','Automation / jobs','Artifacts / fencing / receipts')],False)
d.save()

d=Diagram('view-logical.svg','Domain and authority model','Profiles are portable. Delegations, runs and operations are bound to an actor and client.')
d.rail(163,'STORED OWNERSHIP',[('org','Organization','Membership + authority epoch'),('project','Project','Owner / tombstone / environment'),('binding','Provider binding','Explicit repository / permissions'),('profile','Approved profile','Immutable version + digests')])
d.rail(444,'DELEGATED EXECUTION',[('session','Browser session','24-hour maximum / revocable'),('delegation','OAuth family','Parent session / exact client'),('run','Run grant','One repository / pinned commit'),('operation','Operation + receipt','Fenced attempt / private result')])
d.edge('binding','run','scope intersection')
d.save()

d=Diagram('view-process.svg','Admission, transactions and effects','No network call runs while authority/database locks are held.')
d.rail(163,'ADMIT + COMMIT',[('authenticate','Authenticate','Stored session / delegation'),('authority','Current authority','Ordered locks / deny closed','security'),('reserve','Persist admission','Budget / attempt / audit / outbox'),('commit','Commit transaction','Only then dispatch','store')])
d.rail(444,'EXTERNAL OUTCOME + AUTHORIZED DELIVERY',[('return','Return permitted data','Completed call ≠ delivery right'),('recheck','Reauthorize retrieval','Current client / run / expiry','security'),('persist','Commit outcome','Receipt + encrypted result'),('dispatch','Execute external read','Per-ticket broker recheck')],False)
d.edge('commit','dispatch','dispatch')
for a,b in [('dispatch','persist'),('persist','recheck'),('recheck','return')]: d.edge(a,b)
d.save()

d=Diagram('view-development.svg','Repository and extension boundaries','One canonical project, modular Go core, explicit harness/provider adapters.')
d.rail(163,'PRODUCT SOURCE',[('backend','backend/','Go / Gin / migrations / commands'),('frontend','frontend/','React / Field Twist / widget'),('adapters','sdks/ + integrations/','Existing client adapters'),('deploy','deploy/','Control / runner references')],False)
d.rail(444,'ENGINEERING SUPPORT',[('docs','docs/','Architecture / decisions / evidence'),('scripts','scripts/','Contracts / tests / diagrams'),('tests','tests/','Synthetic acceptance fixtures'),('ci','.github/workflows/','Tests / scans / no deployment')],False)
d.save()

d=Diagram('view-physical.svg','Self-hosted reference topology','This is an installation reference, not an accepted production deployment.')
d.rail(163,'CONTROL HOST · app.keepsave.draveniq.dev',[('tls','TLS + frontend','Same-origin / public runner404'),('api','API replicas','Go / Gin / current authority'),('pg','PostgreSQL + worker','Shared budgets / audit / fences','store'),('storage','Transit / private storage','Keys / backups / ephemeral results','security')],False)
d.edge('tls','api','HTTPS'); d.edge('api','pg','SQL')
d.rail(444,'RUNNER HOST · private control connectivity',[('container','Connector container','RO filesystem / no network','runner'),('podman','Rootless Podman','CPU / memory / PID controls','runner'),('supervisor','Supervisor identity','Enrollment key outside container','runner'),('listener','Private API listener','TLS 1.3 / exact mTLS identity','security')],False)
d.edge('podman','container','exec');d.edge('supervisor','podman','engine');d.edge('supervisor','listener','mTLS')
d.save()

d=Diagram('view-scenario-mcp.svg','Repository-review operation','GitHub credentials stay inside the broker; returned repository content reaches the client/model.')
d.rail(163,'START A CLIENT-BOUND REVIEW',[('client','Approved client','Own delegation / portable profile'),('prepare','Preparing run','Authorize + persist resolution'),('activate','Activate run','Recheck / commit-bound scope'),('enqueue','Admit operation','Request key / budget / receipt')])
d.rail(444,'EXECUTE AND READ THE RESULT',[('relay','Connector relay','Unix socket / one-use ticket','runner'),('broker','Broker dispatch','Recheck / GitHub token custody','security'),('result','Encrypted outcome','Bounded / short-lived / fenced','store'),('retrieve','Result retrieval','Current authority / run expiry')])
d.edge('enqueue','retrieve','status / result')
d.save()

d=Diagram('flow-oauth.svg','Delegated MCP authorization','KeepSave delegation tokens authorize KeepSave; they are separate from GitHub credentials.')
d.rail(163,'AUTHORIZE',[('discovery','Discover issuer\n/ resource','Exact canonical /mcp audience'),('client','Registered public client','Exact callback / S256 PKCE'),('consent','Human consent','Current tracked browser session'),('code','Authorization code','60 seconds / one-time / resource')])
d.rail(444,'USE · refresh only when needed',[('exchange','Atomic code exchange','PKCE + client + resource'),('access','Opaque access token','10-minute maximum'),('validate','Protected MCP use','Current authority / replay revoke'),('refresh','Rotate refresh family','8 hours / bounded parent session')],False)
d.edge('exchange','access');d.edge('access','validate');d.edge('validate','refresh','optional refresh')
# Code exchange and protected use are distinct; refresh is optional, not a prerequisite.
d.save()

d=Diagram('flow-promotion.svg','Versioned environment promotion','Promotion and rollback use the shared vault transaction and retained encryption keys.')
d.rail(163,'REVIEW AND AUTHORIZE',[('source','Alpha / UAT source','Current permission + revision'),('diff','Diff + snapshot','Exact source digest / key version'),('approval','Protected approval','Eligible approver ≠ requester'),('apply','Apply to destination','Current authority / source digest')])
d.rail(444,'ATOMIC JOURNAL + LATER RESTORATION',[('mutation','Encrypt current values','Existing AES-256-GCM format','security'),('journal','Append revision','Expected revision / retained keys','store'),('audit','Audit + outbox','Commit with vault mutation','store'),('restore','Rollback / restore','Append new current revision')])
# Application and restoration each enter the shared transaction-aware vault.
d.save()
