"""Validate a local Linux binary against synthetic PostgreSQL data in isolated Docker containers.

Run on Linux with existing local images. Never pulls images, publishes ports,
mounts production data or connects to a production database. Temporary containers,
network and data are removed in finally; the JSON report contains no credentials.
"""
import argparse
import json
import os
from pathlib import Path
import secrets
import shutil
import subprocess
import tempfile
import time
import urllib.error
import urllib.request
import uuid


def docker(*args):
    return subprocess.check_output(['docker', *args], stderr=subprocess.PIPE).decode().strip()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--binary', required=True)
    parser.add_argument('--base-image', required=True)
    parser.add_argument('--postgres-image', required=True)
    parser.add_argument('--report', required=True)
    args = parser.parse_args()
    binary = Path(args.binary).resolve(strict=True)
    # Require exact local images. No implicit Docker pull is permitted.
    base = json.loads(docker('image', 'inspect', args.base_image))[0]['Id']
    postgres = json.loads(docker('image', 'inspect', args.postgres_image))[0]['Id']
    name = 'apiload-native-check-' + uuid.uuid4().hex[:12]
    scratch = Path(tempfile.mkdtemp(prefix=name + '-', dir='/var/tmp')).resolve()
    os.chmod(scratch, 0o755)
    data = scratch / 'data'
    data.mkdir(mode=0o700)
    os.chown(data, 10001, 10001)
    candidate = scratch / 'candidate'
    candidate.mkdir(mode=0o755)
    shutil.copyfile(binary, candidate / 'gpt-load')
    os.chmod(candidate / 'gpt-load', 0o755)
    admin = 'synthetic-' + secrets.token_hex(24)
    password = secrets.token_hex(24)
    network_created = False
    containers = []
    report = {'scope': 'synthetic PostgreSQL upgrade and Linux container', 'status': 'failed', 'base_image': base, 'postgres_image': postgres}
    stage = 'create_network'
    try:
        docker('network', 'create', '--internal', '--label', 'apiload.acceptance=true', name)
        network_created = True
        pg = name + '-pg'
        containers.append(pg)
        docker('run', '-d', '--name', pg, '--network', name, '--network-alias', 'fixture-db',
               '--memory', '384m', '--cpus', '0.5', '--pids-limit', '128', '--security-opt', 'no-new-privileges',
               '--tmpfs', '/var/lib/postgresql/data:rw,size=256m', '-e', 'POSTGRES_USER=fixture',
               '-e', 'POSTGRES_DB=fixture', '-e', 'POSTGRES_PASSWORD=' + password, postgres)
        stage = 'postgres_ready'
        for _ in range(90):
            if subprocess.run(['docker', 'exec', pg, 'pg_isready', '-h', '127.0.0.1', '-U', 'fixture'], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode == 0:
                break
            time.sleep(.5)
        else:
            raise RuntimeError('database readiness timeout')

        def start_app(suffix, use_candidate):
            app = name + '-' + suffix
            containers.append(app)
            command = ['run', '-d', '--name', app, '--network', name, '--memory', '384m', '--cpus', '0.5',
                       '--pids-limit', '128', '--read-only', '--cap-drop', 'ALL', '--security-opt', 'no-new-privileges',
                       '--tmpfs', '/tmp:rw,size=64m,uid=10001,gid=10001', '-v', str(data) + ':/fixture-data',
                       '-e', 'HOST=0.0.0.0', '-e', 'PORT=3000', '-e', 'DATA_DIR=/fixture-data', '-e', 'MODELS_DEV_AUTO_SYNC_ENABLED=false',
                       '-e', 'AUTH_KEY=' + admin, '-e', 'DATABASE_DSN=postgresql://fixture:' + password + '@fixture-db:5432/fixture?sslmode=disable']
            if use_candidate:
                command += ['-v', str(candidate) + ':/candidate:ro', '--entrypoint', '/candidate/gpt-load']
            docker(*command, base)
            address = json.loads(docker('inspect', app))[0]['NetworkSettings']['Networks'][name]['IPAddress']
            url = 'http://' + address + ':3000'
            for _ in range(120):
                try:
                    with urllib.request.urlopen(url + '/health', timeout=2) as response:
                        health = json.load(response)
                    return app, url, health
                except (urllib.error.URLError, TimeoutError):
                    if not json.loads(docker('inspect', app))[0]['State']['Running']:
                        raise RuntimeError('isolated app exited') from None
                    time.sleep(.5)
            raise RuntimeError('app readiness timeout')

        def call(url, path, body=None, token=None, method=None):
            request = urllib.request.Request(url + path, data=None if body is None else json.dumps(body).encode(),
                      headers={'Authorization': 'Bearer ' + (token or admin), 'Content-Type': 'application/json', 'Idempotency-Key': str(uuid.uuid4())}, method=method)
            with urllib.request.urlopen(request, timeout=15) as response:
                result = json.load(response)
            if path.startswith('/api/'):
                if result.get('code') != 0:
                    raise RuntimeError('control API rejected fixture')
                return result['data']
            return result

        stage = 'old_version_seed'
        old, url, health = start_app('old', False)
        report['old_health'] = health
        group = call(url, '/api/groups', {'name': 'Synthetic upgrade fixture', 'channel_id': 'cohere', 'connection_type': 'api_key',
                     'params': {}, 'models': [{'id': model, 'alias_enabled': False} for model in ('rerank-v3.5', 'embed-v4.0', 'rerank-v4.0')], 'credentials': 'synthetic-upgrade-provider-key'})
        group_id = group['group_id']
        credential = call(url, f'/api/groups/{group_id}/credentials?page=1&page_size=20')['items'][0]['credential_id']
        access = call(url, '/api/access-keys', {'name': 'Synthetic upgrade token', 'filters': {'groups_restricted': True, 'groups': [group_id], 'protocols': ['rerank'], 'models': ['rerank-v3.5']}})
        prices = call(url, '/api/model-prices?page=1&page_size=100')['items']
        ids = {row['model_id']: row['id'] for row in prices if row['channel_id'] == 'cohere'}
        source, linked, copied = (ids[model] for model in ('rerank-v3.5', 'embed-v4.0', 'rerank-v4.0'))

        def fixed_price(value):
            return {'billing_unit': 'request', 'request_price': value, 'input': None, 'output': None,
                    'cache_read': None, 'cache_write': None, 'context_tiers': [], 'mode_schedules': {}, 'confirm_unpriced': True}

        call(url, f'/api/model-prices/{source}', fixed_price('0.0025'), method='PUT')
        docker('stop', '-t', '10', old)
        stage = 'candidate_migration'
        new, url, health = start_app('new', True)
        report['candidate_health'] = health
        current = call(url, f'/api/groups/{group_id}/credentials?page=1&page_size=20')['items']
        assert len(current) == 1 and current[0]['credential_id'] == credential
        revealed = call(url, f'/api/groups/{group_id}/credentials/{credential}/reveal', {})['credential']
        if isinstance(revealed, str):
            revealed = json.loads(revealed)
        assert revealed['api_key'] == 'synthetic-upgrade-provider-key'
        revealed.clear()
        catalog = call(url, '/v1/models', token=access['key'])
        assert any(row['id'] == 'rerank-v3.5' for row in catalog['data'])
        columns = docker('exec', pg, 'psql', '-U', 'fixture', '-d', 'fixture', '-Atc',
                         "SELECT column_name || ':' || is_nullable FROM information_schema.columns WHERE table_name='request_logs' AND column_name IN ('measured_units','service_evidence') ORDER BY column_name")
        assert columns.splitlines() == ['measured_units:YES', 'service_evidence:YES']
        stage = 'price_reference_roundtrip'
        retained = call(url, f'/api/model-prices/{source}')['price']
        assert retained['billing_unit'] == 'request' and retained['request_price'] == '0.0025' and not retained.get('source_price_id')
        call(url, '/api/model-prices/batch', {'ids': [linked], 'mode': 'link', 'reference': {'price_id': source}})
        call(url, '/api/model-prices/batch', {'ids': [copied], 'mode': 'copy', 'price': fixed_price('0.0025')})
        for value in ('0.005', '0', None):
            call(url, f'/api/model-prices/{source}', fixed_price(value), method='PUT')
            followed = call(url, f'/api/model-prices/{linked}')['price']
            independent = call(url, f'/api/model-prices/{copied}')['price']
            assert followed['billing_unit'] == 'request' and followed['request_price'] == value and followed['source_price_id'] == source
            assert independent['request_price'] == '0.0025' and not independent.get('source_price_id')
        # Restart verifies that linkage and unknown values survive reload from PostgreSQL.
        docker('stop', '-t', '10', new)
        _, url, _ = start_app('restarted', True)
        followed = call(url, f'/api/model-prices/{linked}')['price']
        assert followed['source_price_id'] == source and followed['request_price'] is None
        report.update(status='passed', retained_group_id=group_id, retained_credential=True, decryption_preserved=True,
                      retained_access_token=True, nullable_columns=columns.splitlines(), retained_request_price=True,
                      price_reference_updates=True, independent_copy=True, zero_and_unknown_distinct=True, restart_preserved=True)
    except Exception as error:
        report.update(stage=stage, error_type=type(error).__name__)
    finally:
        cleanup = []
        for container in reversed(containers):
            if subprocess.run(['docker', 'rm', '-f', container], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode:
                cleanup.append('container')
        if network_created and subprocess.run(['docker', 'network', 'rm', name], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode:
            cleanup.append('network')
        if scratch.parent != Path('/var/tmp') or not scratch.name.startswith(name + '-'):
            raise RuntimeError('invalid cleanup path')
        shutil.rmtree(scratch)
        report['cleanup'] = 'complete' if not cleanup else cleanup
        Path(args.report).write_text(json.dumps(report, indent=2) + '\n')
        print(json.dumps(report))
    return 0 if report['status'] == 'passed' and report['cleanup'] == 'complete' else 1


if __name__ == '__main__':
    raise SystemExit(main())
