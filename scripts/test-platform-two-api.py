#!/usr/bin/env python3
"""Synthetic HTTP acceptance against two explicitly local API processes."""
import argparse
import json
import time
import urllib.error
import urllib.request
import uuid
from pathlib import Path

PORTS = (4770, 4772)

def call(port, method, path, body=None, token=None, expect=200):
    assert port in PORTS
    headers = {'Content-Type': 'application/json'}
    if token: headers['Authorization'] = 'Bearer ' + token
    req = urllib.request.Request(f'http://127.0.0.1:{port}/api/v1{path}', data=json.dumps(body).encode() if body is not None else None, headers=headers, method=method)
    try: response = urllib.request.urlopen(req, timeout=15)
    except urllib.error.HTTPError as error: response = error
    with response:
        payload = response.read(4 * 1024 * 1024)
        assert response.status == expect, f'{method} {path}: HTTP {response.status}, wanted {expect}'
        return json.loads(payload) if payload else None

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--context', type=Path, help='Optional output file for nonsecret synthetic browser fixture metadata')
    args = parser.parse_args()
    started = time.monotonic()
    suffix = uuid.uuid4().hex[:8]
    email = f'local-platform-owner-{suffix}@example.invalid'
    password = 'Local-fixture-2026!'
    owner = call(PORTS[0], 'POST', '/auth/register', {'email': email, 'password': password}, expect=201)
    other = call(PORTS[1], 'POST', '/auth/register', {'email': f'local-platform-other-{suffix}@example.invalid', 'password': password}, expect=201)
    token = owner['token']
    project = call(PORTS[0], 'POST', '/projects', {'name': 'Platform acceptance', 'description': 'Synthetic local fixture'}, token, 201)['project']
    path = '/projects/' + project['id']
    secret = call(PORTS[1], 'POST', path + '/secrets', {'key': 'REVIEW_CREDENTIAL', 'value': 'local-canary-v1', 'environment': 'alpha'}, token, 201)['secret']
    sp = path + '/secrets/' + secret['id']
    life = call(PORTS[0], 'PUT', sp + '/lifecycle', {'responsible_user_id': owner['user']['id'], 'declared_expires_at': '2026-10-09T00:00:00Z', 'renewal_at': None, 'provenance': 'Synthetic declared renewal', 'expected_revision': 0}, token)
    assert life['revision'] == 1
    listed = call(PORTS[1], 'GET', path + '/secret-lifecycle', token=token)
    assert len(listed['records']) == 1 and 'value' not in json.dumps(listed)
    call(PORTS[0], 'PUT', sp, {'value': 'local-canary-v2', 'expected_revision': 1}, token)
    history = call(PORTS[1], 'GET', sp + '/versions', token=token)
    assert 'local-canary' not in json.dumps(history)
    bundle = call(PORTS[0], 'POST', path + '/backups', token=token, expect=201)
    assert bundle['format'] == 'keepsave.encrypted-vault.v2'
    verified = call(PORTS[1], 'POST', path + '/backups/verify', bundle, token)
    assert verified['lifecycle_records'] == 1
    call(PORTS[0], 'POST', path + '/rotate-keys', token=token)
    old = call(PORTS[1], 'GET', sp + '/versions/1', token=token)
    assert 'local-canary-v1' in json.dumps(old)
    current = call(PORTS[1], 'GET', sp, token=token)['secret']['revision']
    call(PORTS[1], 'POST', sp + '/versions/1/restore', {'expected_current_revision': current}, token)
    call(PORTS[0], 'GET', sp, token=other['token'], expect=403)
    call(PORTS[1], 'GET', path + '/secrets/' + str(uuid.uuid4()), token=other['token'], expect=403)
    sessions = call(PORTS[1], 'GET', '/account/sessions', token=token)['sessions']
    current = next(s for s in sessions if s['current'])
    call(PORTS[0], 'DELETE', '/account/sessions/' + current['id'], token=token, expect=204)
    call(PORTS[1], 'GET', path + '/secret-lifecycle', token=token, expect=401)
    fresh = call(PORTS[1], 'POST', '/auth/login', {'email': email, 'password': password})
    assert fresh['token'] != token
    call(PORTS[0], 'GET', path + '/secret-lifecycle', token=fresh['token'])
    print(json.dumps({'status': 'passed', 'api_processes': 2, 'checks': ['tracked-session cross-instance revocation', 'foreign and missing secret denial', 'lifecycle metadata without values', 'vault history/restore', 'rotation retains historical access', 'v2 backup verification'], 'elapsed_seconds': round(time.monotonic() - started, 3), 'capacity_claim': False, 'live_providers': False}))
    if args.context:
        args.context.write_text(json.dumps({'project_id': project['id'], 'email': email}))

if __name__ == '__main__': main()
