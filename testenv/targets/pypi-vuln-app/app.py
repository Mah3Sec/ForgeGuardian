# FORGEGUARDIAN TEST FIXTURE — intentionally vulnerable Python patterns
# Semgrep + behavioral scanner should flag all of these.

import yaml
import subprocess
import os
import pickle
import sqlite3

# Pattern 1: yaml.load() with no Loader — CVE-2020-14343 trigger
def parse_config(config_str):
    # Semgrep rule: dangerous-yaml-load
    return yaml.load(config_str)  # should be yaml.safe_load()

# Pattern 2: subprocess with shell=True + user input
def run_command(user_cmd):
    # Semgrep rule: subprocess-shell-true
    return subprocess.check_output(user_cmd, shell=True)

# Pattern 3: pickle deserialization of untrusted data
def load_session(session_bytes):
    # Semgrep rule: dangerous-pickle-loads
    return pickle.loads(session_bytes)

# Pattern 4: SQL injection via f-string
def get_user(conn, username):
    cursor = conn.cursor()
    # Semgrep rule: sql-injection-string-format
    cursor.execute(f"SELECT * FROM users WHERE name = '{username}'")
    return cursor.fetchone()

# Pattern 5: eval of user input
def calc(expr):
    # Semgrep rule: dangerous-eval
    return eval(expr)

# Pattern 6: hardcoded credential
AWS_SECRET = "AKIAIOSFODNN7EXAMPLE"  # Semgrep: hardcoded-aws-key
DB_PASSWORD = "admin123"             # Semgrep: hardcoded-password

# Pattern 7: os.system with unsanitized input
def ping_host(host):
    os.system(f"ping -c 1 {host}")
