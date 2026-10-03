#!/usr/bin/env python3
"""Exercise the Docker pilot on a disposable tassm-smoke project only.

Start it with HTTP_PORT=18081 HTTPS_PORT=18444 APP_ORIGIN=http://localhost:18081
and `docker compose --env-file .env.pilot -f compose.pilot.yaml -p tassm-smoke up ...`.
Never point this at a real community. Credentials are generated, not logged.
"""
import http.cookiejar
import json
import secrets
import subprocess
import urllib.error
import urllib.parse
import urllib.request

BASE = 'http://localhost:18081'
COMPOSE = ['docker', 'compose', '--env-file', '.env.pilot', '-f', 'compose.pilot.yaml', '-p', 'tassm-smoke']


def admin(*args):
    return subprocess.check_output(COMPOSE + ['exec', '-T', 'api', '/app/admin', *args], text=True)


def client():
    return urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))


def request(c, method, path, data=None, status=200, origin=BASE):
    headers = {'Origin': origin}
    if data is not None:
        headers['Content-Type'] = 'application/json'
    req = urllib.request.Request(BASE + '/api' + path, data=None if data is None else json.dumps(data).encode(), headers=headers, method=method)
    try:
        res = c.open(req)
    except urllib.error.HTTPError as e:
        res = e
    raw = res.read()
    assert res.status == status, f'{method} {path}: expected {status}, got {res.status}: {raw[:160]!r}'
    return json.loads(raw) if raw else None


def token(output):
    # CLI intentionally prints a one-use link for the operator, captured here only.
    for word in output.split():
        if '#token=' in word:
            return urllib.parse.parse_qs(urllib.parse.urlsplit(word).fragment)['token'][0]
    raise AssertionError('admin did not return a token link')


def main():
    nonce = secrets.token_hex(4)
    people = []
    anonymous = client()
    request(anonymous, 'GET', '/feed', status=401)
    request(anonymous, 'POST', '/auth/login', {'email': 'nobody@example.invalid', 'password': 'wrong'}, status=403, origin='https://evil.example')
    for n in range(3):
        email, username = f'smoke-{nonce}-{n}@example.invalid', f'smoke_{nonce}_{n}'
        password = secrets.token_urlsafe(24)
        invitation = token(admin('invite', '--admin', email) if n == 0 else admin('invite', email))
        c = client()
        payload = dict(token=invitation, email=email, username=username, password=password, displayName=f'Testperson {n+1}')
        user = request(c, 'POST', '/auth/register', payload)['user']
        request(client(), 'POST', '/auth/register', payload, status=400)
        assert request(c, 'GET', '/auth/me')['user']['id'] == user['id']
        people.append((c, user, email, password))
    a, b, c = [p[0] for p in people]
    aid, bid, cid = [p[1]['id'] for p in people]
    public = request(a, 'POST', '/posts', {'text': 'Gemeinsam beginnt hier etwas Neues.', 'visibility': 'public'}, status=201)['post']
    private = request(a, 'POST', '/posts', {'text': 'Nur für enge Freunde.', 'visibility': 'close_friends'}, status=201)['post']
    request(b, 'GET', '/posts/' + private['id'], status=404)
    request(a, 'PUT', '/relationships/close_friend/' + bid, status=204)
    assert request(b, 'GET', '/posts/' + private['id'])['post']['id'] == private['id']
    request(c, 'GET', '/posts/' + private['id'], status=404)
    request(b, 'POST', '/posts/' + public['id'] + '/comments', {'text': 'Ein guter Anfang.'}, status=201)
    request(c, 'DELETE', '/posts/' + public['id'], status=404)
    conv = request(a, 'POST', '/conversations', {'memberIds': [bid]}, status=201)['conversation']
    request(a, 'POST', '/conversations/' + conv['id'] + '/messages', {'text': 'Privat und gemeinsam.'}, status=201)
    assert len(request(b, 'GET', '/conversations/' + conv['id'] + '/messages')['messages']) == 1
    request(c, 'GET', '/conversations/' + conv['id'] + '/messages', status=404)
    request(b, 'PUT', '/relationships/block/' + aid, status=204)
    request(a, 'GET', '/posts/' + private['id'])
    request(b, 'GET', '/posts/' + public['id'], status=404)
    request(a, 'POST', '/conversations/' + conv['id'] + '/messages', {'text': 'Must not send'}, status=403)
    request(b, 'DELETE', '/relationships/block/' + aid, status=204)
    request(b, 'GET', '/posts/' + private['id'], status=404)
    request(a, 'POST', '/auth/logout')
    request(a, 'GET', '/auth/me', status=401)
    request(a, 'POST', '/auth/login', {'email': people[0][2], 'password': people[0][3]})
    reset = token(admin('reset', people[0][2]))
    replacement = secrets.token_urlsafe(24)
    request(client(), 'POST', '/auth/reset', {'token': reset, 'password': replacement})
    request(a, 'GET', '/auth/me', status=401)
    request(client(), 'POST', '/auth/login', {'email': people[0][2], 'password': people[0][3]}, status=401)
    request(a, 'POST', '/auth/login', {'email': people[0][2], 'password': replacement})
    admin('disable', people[2][2])
    request(c, 'GET', '/feed', status=401)
    print('Pilot smoke passed: invitations, origin checks, visibility, comments, blocks, messages, logout, reset, disable.')


if __name__ == '__main__':
    main()
